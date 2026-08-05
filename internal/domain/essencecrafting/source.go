package essencecrafting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const sourceFile = "essenceCrafting.v2.json"

type sourceEnhancement struct {
	Name         string         `json:"name"`
	MinimumLevel int            `json:"minItemLevel"`
	Bound        *sourceRecipe  `json:"bound"`
	Unbound      *sourceRecipe  `json:"unbound"`
	Prefix       []string       `json:"prefix"`
	Suffix       []string       `json:"suffix"`
	Extra        []string       `json:"extra"`
	Effects      []sourceEffect `json:"enchantments"`
}

type sourceRecipe struct {
	RecipeID     int                 `json:"recipeId"`
	Level        int                 `json:"level"`
	Essence      int                 `json:"essence"`
	Collectibles []sourceRequirement `json:"collectible"`
}

type sourceRequirement struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type sourceEffect struct {
	Name         string           `json:"name"`
	Bonus        string           `json:"bonus"`
	ModifierDice string           `json:"modifierDice"`
	Modifiers    []sourceModifier `json:"modifiers"`
}

type sourceModifier struct {
	Level int         `json:"level"`
	Value json.Number `json:"value"`
}

func loadSource(root string) ([]sourceEnhancement, error) {
	path := filepath.Join(root, sourceFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read Essence Crafting source %s: %w", path, err)
	}
	if err := rejectDuplicateKeys(data); err != nil {
		return nil, fmt.Errorf("decode Essence Crafting source %s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	var result []sourceEnhancement
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Essence Crafting source %s: %w", path, err)
	}
	if err := requireEOF(decoder); err != nil {
		return nil, fmt.Errorf("decode Essence Crafting source %s: %w", path, err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("Essence Crafting source %s contains no enhancements", path)
	}
	return result, nil
}

func requireEOF(decoder *json.Decoder) error {
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return err
	}
	return nil
}

// encoding/json accepts duplicate object keys. The source is a canonical
// input, so reject them before the typed strict decode can discard one.
func rejectDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	return requireEOF(decoder)
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok {
				return fmt.Errorf("object key is not a string")
			}
			if _, exists := seen[name]; exists {
				return fmt.Errorf("duplicate object key %q", name)
			}
			seen[name] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}
