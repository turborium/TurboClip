package text

import (
	"fmt"
	"log"

	toml "github.com/BurntSushi/toml"
)

var textDict = map[string]string{}

func LoadFromFile(filename string) error {
	dict := map[string]interface{}{}

	_, err := toml.DecodeFile(filename, &dict)
	if err != nil {
		return fmt.Errorf("bad file \"%s\": %v", filename, err)
	}

	for k, v := range dict {
		if s, ok := v.(string); ok {
			textDict[k] = s
		} else {
			return fmt.Errorf("bad value (not a string): {\"%s\": \"%v\"}", k, v)
		}
	}

	return nil
}

// Format is like fmt.Sprintf, but using language-specific formatting.
func Format(key string, a ...interface{}) string {

	if format, ok := textDict[key]; ok {
		return fmt.Sprintf(format, a...)
	} else {
		log.Printf("WARNING[text] Key \"%s\" not found!", key)
		return fmt.Sprint("[" + key + "]")
	}
}
