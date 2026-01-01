package util

func BoolToOnOff(value bool) string {
	if value {
		return "on"
	}

	return "off"
}
