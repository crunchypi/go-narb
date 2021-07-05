/*
File contains some helper funcs that aren't explicitly related to sessionmembers.
*/
package sessionmember

// containsStr checks if a string slice contains a string.
func containsStr(s string, ss []string) bool {
	for i := 0; i < len(ss); i++ {
		if s == ss[i] {
			return true
		}
	}
	return false
}

// maxMapVal finds the maximum value (as opposed to key) in a map[string]int.
// will return false if there are more than one max values.
func maxMapVal(m map[string]int) (string, bool) {
	if len(m) == 0 {
		return "", false
	}
	maxKeys := make([]string, 0, len(m)/2)
	maxVal := 0
	for k, v := range m {
		switch {
		case v > maxVal:
			maxKeys = make([]string, 0, len(m)/2)
			maxKeys = append(maxKeys, k)
			maxVal = v
		case v == maxVal:
			maxKeys = append(maxKeys, k)
		}
	}
	return maxKeys[0], len(maxKeys) == 1
}
