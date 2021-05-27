package m3u8

func Init() {
	CreateDirIfNotExists(DownloadDir)
	CreateDirIfNotExists(WorkDir)
}
