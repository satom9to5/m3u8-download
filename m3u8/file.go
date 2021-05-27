package m3u8

import (
	"os"
	"strings"
)

func CreateDirIfNotExists(dir string) {
	if _, err := os.Stat(WorkDir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0777)
	}
}

func ReplaceEscapeChar(str string) string {
	str = strings.Replace(str, "/", "／", -1)
	str = strings.Replace(str, "\\!", "!", -1)
	str = strings.Replace(str, "\\", "￥", -1)
	str = strings.Replace(str, "?", "？", -1)
	return strings.Replace(str, ":", "：", -1)
}
