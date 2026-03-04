package conv

import "time"

func ToEntUserBirthday(v int64) time.Time {
	return time.Unix(v, 0)
}
