package permissions

type Choice int

const (
	ChoiceAllow Choice = iota
	ChoiceAllowSession
	ChoiceDeny
	ChoiceAskWhatToDo
)

func Choices(isDangerous bool) []string {
	if isDangerous {
		return []string{"Allow", "Deny", "Ask what to do instead"}
	}
	return []string{"Allow", "Allow for this session", "Deny", "Ask what to do instead"}
}

func ChoiceAt(cursor int, isDangerous bool) Choice {
	if isDangerous {
		switch cursor {
		case 0:
			return ChoiceAllow
		case 1:
			return ChoiceDeny
		default:
			return ChoiceAskWhatToDo
		}
	}
	switch cursor {
	case 0:
		return ChoiceAllow
	case 1:
		return ChoiceAllowSession
	case 2:
		return ChoiceDeny
	default:
		return ChoiceAskWhatToDo
	}
}
