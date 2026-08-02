package parser

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"

	"yourddo-data-tools/v2/internal/compendium/cleanup"
	"yourddo-data-tools/v2/internal/dataset"
)

// List of core template names (mirrored from the context for quick reference)
var coreTemplateNames = []string{
	"{{Template:Item|", "{{Template:Shield|", "{{Template:Material|", "{{Template:Augment|", "{{Template:Weapon|",
	"{{Template:Armor|", "{{Template:Consumable|", "{{Template:Cosmetic|", "{{Template:RuneArm|", "{{Template:SpellCaster|", "{{Template:VIPLoyalty|",
	"{{Template:Quiver|", "{{Template:Filigree|",
	"{{Template:Trick|", "{{Item|", "{{Shield|", "{{Material|", "{{Augment|", "{{Armor|", "{{VIPLoyalty|",
	"{{Consumable|", "{{Cosmetic|", "{{RuneArm|", "{{Trick|", "{{Weapon|", "{{SpellCaster|", "{{Quiver|", "{{Filigree|",
}

func romanToInt(s string) int {
	romanMap := map[byte]int{
		'I': 1,
		'V': 5,
		'X': 10,
		'L': 50,
		'C': 100,
		'D': 500,
		'M': 1000,
	}

	result := 0
	n := len(s)

	for i := 0; i < n; i++ {
		currentVal := romanMap[s[i]]

		// Check for subtractive cases
		if i+1 < n && currentVal < romanMap[s[i+1]] {
			result -= currentVal
		} else {
			result += currentVal
		}
	}
	return result
}

func intToRoman(n int) string {
	if n <= 0 {
		return ""
	}

	values := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	symbols := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}

	var result strings.Builder
	for i := 0; i < len(values); i++ {
		for n >= values[i] {
			n -= values[i]
			result.WriteString(symbols[i])
		}
	}

	return result.String()
}

var preUSuffixRegex = regexp.MustCompile(`\(Pre U\d+\)$`)

// ProcessContentMap orchestrates the parsing of all raw pages.
func ProcessContentMap(rawContentMap map[string]string) []dataset.ItemData {
	var processedItems []dataset.ItemData

	for title, rawContent := range rawContentMap {
		if preUSuffixRegex.MatchString(title) {
			logrus.Debugf("Skipping %s (Pre-Update item)\n", title)
			continue
		}

		if strings.Contains(strings.ToLower(rawContent), "{{discontinued}}") || strings.Contains(strings.ToLower(rawContent), "{{discontinued|") {
			logrus.Debugf("Skipping %s (Discontinued item)\n", title)
			continue
		}

		if strings.Contains(strings.ToLower(rawContent), "{{starter}}") {
			logrus.Debugf("Skipping %s (Starter item)\n", title)
			continue
		}

		cleanedContent := cleanup.CleanRawContent(rawContent)
		fields, err := parseTemplateFields(cleanedContent)

		if err != nil {
			if !strings.Contains(err.Error(), "redirect or empty") {
				logrus.Debugf("Skipping %s (Error parsing core template): %v\n", title, err)
			}
			continue
		}

		// Only convert to ItemData if parsing was successful and it looks like an item
		if fields["type"] != "" ||
			strings.Contains(strings.ToLower(cleanedContent), "{{item|") ||
			strings.Contains(strings.ToLower(cleanedContent), "{{quiver|") ||
			strings.Contains(strings.ToLower(cleanedContent), "{{template:quiver|") {
			item := ConvertItemToJSON(title, fields) // Calls the converter in parser/converter.go

			// Check if the item is marked as discontinued via its drop locations
			isDiscontinued := false
			for _, loc := range item.DropLocations {
				if loc.SourceType == "Discontinued" {
					isDiscontinued = true
					break
				}
			}
			if isDiscontinued {
				logrus.Debugf("Skipping %s (Discontinued via DropLocation)\n", title)
				continue
			}

			processedItems = append(processedItems, item)
		}
	}

	return processedItems
}

// parseTemplateFields extracts and parses the core template using a brace-counting parser.
func parseTemplateFields(rawContent string) (map[string]string, error) {
	if strings.Contains(rawContent, "#REDIRECT") || strings.TrimSpace(rawContent) == "" {
		return nil, fmt.Errorf("page is a redirect or empty")
	}

	aggressiveContent := strings.ToLower(rawContent)
	var templateStart = -1
	var templatePrefix string
	var prefixLength int

	// 1. Find the start of the core template
	for _, name := range coreTemplateNames {
		start := strings.Index(aggressiveContent, strings.ToLower(name))
		if start != -1 {
			templateStart = start
			templatePrefix = name
			prefixLength = len(name)
			break
		}
	}

	if templateStart == -1 {
		return nil, fmt.Errorf("could not find a matching core template in the content")
	}

	// Get the index in the original, full content string
	rawTemplateStart := strings.Index(rawContent, templatePrefix)
	if rawTemplateStart == -1 {
		// Fallback search in case of a slight casing mismatch (shouldn't happen post-cleanup)
		rawTemplateStart = strings.Index(strings.ToLower(rawContent), strings.ToLower(templatePrefix))
		if rawTemplateStart == -1 {
			return nil, fmt.Errorf("could not locate clean prefix in raw content")
		}
	}

	// 2. Find the matching end of this core template. Pages commonly contain
	// additional templates after it, so the final closing braces in the page
	// are not necessarily the end of the record being parsed.
	paramListStart := rawTemplateStart + prefixLength
	templateEnd := -1
	type templateFrame struct {
		name   string
		offset int
	}
	stack := []templateFrame{{name: strings.TrimSuffix(strings.TrimPrefix(templatePrefix, "{{"), "|"), offset: rawTemplateStart}}
	for i := paramListStart; i+1 < len(rawContent); i++ {
		switch rawContent[i : i+2] {
		case "{{":
			stack = append(stack, templateFrame{name: templateNameAt(rawContent, i), offset: i})
			i++
		case "}}":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			if len(stack) == 0 {
				templateEnd = i
			}
			i++
		}
		if templateEnd != -1 {
			break
		}
	}
	if templateEnd == -1 {
		if len(stack) > 1 {
			unclosed := stack[len(stack)-1]
			return nil, fmt.Errorf(
				"core template %q contains unclosed nested template %q at byte %d near %q",
				stack[0].name, unclosed.name, unclosed.offset, sourceExcerpt(rawContent, unclosed.offset),
			)
		}
		return nil, fmt.Errorf(
			"core template %q at byte %d has no matching closing braces near %q",
			stack[0].name, rawTemplateStart, sourceExcerpt(rawContent, rawTemplateStart),
		)
	}

	// 3. Extract the parameter list string
	paramList := rawContent[paramListStart:templateEnd]

	fields := make(map[string]string)

	// 4. CORE: Brace-counting loop
	var currentKey string
	var currentVal strings.Builder
	var braceCount = 0
	var valueStarted = false

	for i, char := range paramList {
		if char == '{' {
			braceCount++
		} else if char == '}' {
			braceCount--
			if braceCount < 0 {
				offset := paramListStart + i
				if nested := lastClosedNestedTemplate(rawContent, paramListStart, offset); nested != "" {
					return nil, fmt.Errorf(
						"core template %q has an unmatched closing brace at byte %d immediately after nested template %q; source wikitext contains an extra '}' near %q (the browser renderer may tolerate this malformed nested-template syntax)",
						strings.TrimSuffix(strings.TrimPrefix(templatePrefix, "{{"), "|"), offset, nested, sourceExcerpt(rawContent, offset),
					)
				}
				return nil, fmt.Errorf(
					"core template %q has an unmatched closing brace at byte %d near %q",
					strings.TrimSuffix(strings.TrimPrefix(templatePrefix, "{{"), "|"), offset, sourceExcerpt(rawContent, offset),
				)
			}
		}

		if char == '|' && braceCount == 0 {
			if valueStarted {
				value := strings.TrimSpace(currentVal.String())

				if currentKey != "" {
					fields[currentKey] = value
				}

				currentKey = ""
				currentVal.Reset()
				valueStarted = false
				continue
			}
		}

		if char == '=' && braceCount == 0 && !valueStarted {
			currentKey = strings.TrimSpace(currentVal.String())
			currentVal.Reset()
			valueStarted = true
			continue
		}

		currentVal.WriteRune(char)

		if i == len(paramList)-1 {
			value := strings.TrimSpace(currentVal.String())
			if currentKey != "" && valueStarted {
				fields[currentKey] = value
			}
		}
	}
	if braceCount != 0 {
		return nil, fmt.Errorf("core template contains unmatched nested braces")
	}

	if len(fields) == 0 && strings.TrimSpace(paramList) != "" && !strings.Contains(strings.ToLower(paramList), "category") {
		return nil, fmt.Errorf("failed to parse fields with brace counter")
	}

	return fields, nil
}

func templateNameAt(content string, offset int) string {
	start := offset + 2
	if start >= len(content) {
		return "<unknown>"
	}
	end := start
	for end < len(content) && content[end] != '|' && content[end] != '}' && content[end] != '\n' {
		end++
	}
	name := strings.TrimSpace(content[start:end])
	if name == "" {
		return "<unknown>"
	}
	return name
}

func sourceExcerpt(content string, offset int) string {
	const radius = 60
	start := offset - radius
	if start < 0 {
		start = 0
	}
	end := offset + radius
	if end > len(content) {
		end = len(content)
	}
	return strings.TrimSpace(content[start:end])
}

func lastClosedNestedTemplate(content string, start, end int) string {
	stack := make([]string, 0)
	lastClosed := ""
	for index := start; index+1 < end; index++ {
		switch content[index : index+2] {
		case "{{":
			stack = append(stack, templateNameAt(content, index))
			index++
		case "}}":
			if len(stack) > 0 {
				lastClosed = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			index++
		}
	}
	return lastClosed
}
