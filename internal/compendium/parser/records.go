package parser

import (
	"fmt"
	"strconv"
	"strings"

	"yourddo-data-tools/internal/compendium/cleanup"
	"yourddo-data-tools/internal/dataset"
)

// MasterExclusionReason implements the source-level exclusions inherited from
// dataSpider. These records are deliberately outside the canonical master
// dataset; this is ingestion policy, not domain-specific filtering.
func MasterExclusionReason(pageTitle, rawContent string) string {
	if preUSuffixRegex.MatchString(strings.TrimSpace(pageTitle)) {
		return "pre-update item"
	}
	lower := strings.ToLower(rawContent)
	if strings.Contains(lower, "{{discontinued}}") || strings.Contains(lower, "{{discontinued|") {
		return "discontinued item"
	}
	if strings.Contains(lower, "{{starter}}") {
		return "starter item"
	}
	return ""
}

// HasDiscontinuedDropLocation catches the legacy secondary exclusion where a
// discontinued marker is represented through the drop-location field rather
// than as an exact raw page marker.
func HasDiscontinuedDropLocation(rawContent string) bool {
	cleaned := cleanup.CleanRawContent(rawContent)
	fields, err := parseTemplateFields(cleaned)
	if err != nil {
		return false
	}
	for _, location := range ParseMultiTemplateDropLocation(fields["droplocation"]) {
		if location.SourceType == "Discontinued" {
			return true
		}
	}
	return false
}

// ParseItemRecord parses one source page without applying product- or
// domain-specific inclusion rules. The caller is responsible for deciding
// which source categories belong in a master dataset.
func ParseItemRecord(pageTitle, rawContent string) (dataset.ItemData, error) {
	pageTitle = strings.TrimSpace(pageTitle)
	if pageTitle == "" {
		return dataset.ItemData{}, fmt.Errorf("source page title is empty")
	}
	cleaned := cleanup.CleanRawContent(rawContent)
	fields, err := parseTemplateFields(cleaned)
	if err != nil {
		return dataset.ItemData{}, fmt.Errorf("parse core item template: %w", err)
	}
	lower := strings.ToLower(cleaned)
	if fields["type"] == "" && fields["capacity"] == "" &&
		!strings.Contains(lower, "{{item|") &&
		!strings.Contains(lower, "{{template:item|") &&
		!strings.Contains(lower, "{{quiver|") &&
		!strings.Contains(lower, "{{template:quiver|") {
		return dataset.ItemData{}, fmt.Errorf("source page does not contain an item template")
	}
	item := ConvertItemToJSON(pageTitle, fields)
	if strings.TrimSpace(item.Name) == "" {
		item.Name = pageTitle
	}
	return item, nil
}

// ParseAugmentRecord parses one source page without injecting synthetic
// augments or filtering records by product policy.
func ParseAugmentRecord(pageTitle, rawContent string) (dataset.AugmentItem, error) {
	pageTitle = strings.TrimSpace(pageTitle)
	if pageTitle == "" {
		return dataset.AugmentItem{}, fmt.Errorf("source page title is empty")
	}
	cleaned := cleanup.CleanRawContent(rawContent)
	lower := strings.ToLower(cleaned)
	if !strings.Contains(lower, "{{augment|") && !strings.Contains(lower, "{{template:augment|") {
		return dataset.AugmentItem{}, fmt.Errorf("source page does not contain an augment template")
	}
	fields, err := parseTemplateFields(cleaned)
	if err != nil {
		return dataset.AugmentItem{}, fmt.Errorf("parse core augment template: %w", err)
	}
	for _, field := range []string{"minlevel", "absoluteminlevel", "hardness", "durability"} {
		if value := strings.TrimSpace(fields[field]); value != "" {
			if _, err := strconv.Atoi(value); err != nil {
				return dataset.AugmentItem{}, fieldValueError(field, value, "an integer", err)
			}
		}
	}
	if value := strings.TrimSpace(fields["weight"]); value != "" {
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return dataset.AugmentItem{}, fieldValueError("weight", value, "a number", err)
		}
	}
	if value := strings.TrimSpace(fields["update"]); value != "" {
		value = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(value), "update"))
		if _, err := strconv.Atoi(value); err != nil {
			return dataset.AugmentItem{}, fieldValueError("update", fields["update"], "an update number", err)
		}
	}
	augment := ConvertAugmentToJSON(pageTitle, fields)
	if strings.TrimSpace(augment.Name) == "" {
		return dataset.AugmentItem{}, fmt.Errorf("canonical augment name is empty")
	}
	return augment, nil
}

func fieldValueError(field, value, expected string, parseErr error) error {
	if name := firstNestedTemplateName(value); name != "" {
		return fmt.Errorf(
			"field %q contains nested template %q whose value was not rendered by the Compendium API (raw value %q); expected %s: %w",
			field, name, value, expected, parseErr,
		)
	}
	return fmt.Errorf("field %q has raw value %q; expected %s: %w", field, value, expected, parseErr)
}

func firstNestedTemplateName(value string) string {
	offset := strings.Index(value, "{{")
	if offset == -1 {
		return ""
	}
	return templateNameAt(value, offset)
}
