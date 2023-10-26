package converting

func ConvertMap(originalMap map[string][]string) map[string]interface{} {
	convertedMap := make(map[string]interface{})

	for key, values := range originalMap {
		convertedMap[key] = interface{}(values)
	}

	return convertedMap
}
