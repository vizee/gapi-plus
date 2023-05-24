package gen

import (
	"bytes"
	"encoding/json"
	"os"

	"gopkg.in/yaml.v2"
)

func convertJsonNumberToType(o map[string]any) error {
	for k, v := range o {
		n, ok := v.(json.Number)
		if !ok {
			continue
		}

		i, err := n.Int64()
		if err == nil {
			o[k] = i
			continue
		}

		f, err := n.Float64()
		if err != nil {
			return err
		}
		o[k] = f
	}

	return nil
}

func marshalJsonToYaml(o any) ([]byte, error) {
	data, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var obj map[string]any
	err = dec.Decode(&obj)
	if err != nil {
		return nil, err
	}
	err = convertJsonNumberToType(obj)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(obj)
}

func loadJsonFile(fname string, obj any) error {
	data, err := os.ReadFile(fname)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, obj)
}

func mergeMap[K comparable, V any](dst map[K]V, src map[K]V) {
	for k, v := range src {
		dst[k] = v
	}
}
