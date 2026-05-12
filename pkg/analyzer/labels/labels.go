package labels

func Match(selector map[string]string, resourceLabels map[string]string) bool {
	for key, expectedValue := range selector {
		actualValue, exists := resourceLabels[key]
		if !exists || actualValue != expectedValue {
			return false
		}
	}
	return true
}
