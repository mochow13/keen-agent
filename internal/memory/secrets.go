package memory

import "regexp"

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(aws_access_key_id|aws_secret_access_key|aws_session_token)`),
	regexp.MustCompile(`(?i)(api[_-]?key|secret|token|password|passwd|pwd)\s*[:=]\s*\S{4,}`),
	regexp.MustCompile(`(?i)\bapikey\s+\S{4,}`),
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9_\-./+=]{10,}`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36}`),
	regexp.MustCompile(`(?i)(xox[bpoa])-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`(?i)sk-[A-Za-z0-9]{20,}`),
}

func ContainsSecret(content string) bool {
	for _, p := range secretPatterns {
		if p.MatchString(content) {
			return true
		}
	}
	return false
}
