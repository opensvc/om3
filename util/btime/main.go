package btime

import "time"

func GetBootTime() (time.Time, error) {
	bt, err := bootTime()
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(int64(bt), 0), nil
}
