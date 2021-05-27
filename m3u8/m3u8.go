package m3u8

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type StreamInfo struct {
	Info       string
	Bandwidth  int
	VideoUri   string
	AudioUri   string
	ctx        context.Context
	cancelFunc context.CancelFunc
}

type audioInfoMap map[string]string

func MaxPriorityStream(uri string) (StreamInfo, error) {
	var err error

	streamInfos, err := ExtractStreamInfo(uri)

	if err != nil {
		return StreamInfo{}, err
	}

	sort.SliceStable(streamInfos, func(i, j int) bool { return streamInfos[i].Bandwidth > streamInfos[j].Bandwidth })

	fmt.Printf("max priority bandwidth: %d\n", streamInfos[0].Bandwidth)
	return streamInfos[0], nil
}

func ExtractStreamInfo(uri string) ([]StreamInfo, error) {
	var err error

	filepath := WorkDir + "/master.m3u8"
	err = RetryDownload(uri, filepath)
	if err != nil {
		return nil, err
	}

	defer func() {
		os.Remove(filepath)
	}()

	bytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.Replace(string(bytes), "\r", "", -1), "\n")

	streamRe := regexp.MustCompile(`EXT-X-STREAM-INF.*BANDWIDTH=(\d+)`)
	audioRe := regexp.MustCompile("EXT-X-MEDIA.*TYPE=AUDIO.*GROUP-ID=\"*([^\"]+)\"*.*URI=\"*([^\"]+)\"*")

	streamInfos := []StreamInfo{}
	audioInfos := audioInfoMap{}

	for index, line := range lines {
		if streamMatches := streamRe.FindStringSubmatch(line); len(streamMatches) > 0 {
			bandwidth, err := strconv.Atoi(streamMatches[1])
			if err == nil {
				streamInfos = append(streamInfos, StreamInfo{
					Info:      line,
					Bandwidth: bandwidth,
					VideoUri:  lines[index+1],
				})
			}
		}

		if audioMatches := audioRe.FindStringSubmatch(line); len(audioMatches) > 0 {
			audioInfos[audioMatches[1]] = audioMatches[2]
		}
	}

	for i, _ := range streamInfos {
		streamInfos[i].AppendAudioInfo(audioInfos)
	}

	return streamInfos, nil
}

func (si *StreamInfo) AppendAudioInfo(audioInfos audioInfoMap) {
	re := regexp.MustCompile(`AUDIO="*([^",]+)`)
	matches := re.FindStringSubmatch(si.Info)
	if len(matches) == 0 {
		return
	}

	if uri, ok := audioInfos[matches[1]]; ok {
		si.AudioUri = uri
	}
}

func (si *StreamInfo) Download(title string) error {
	if si.ctx == nil {
		si.ctx = context.Background()
	}

	title = ReplaceEscapeChar(title)

	if si.AudioUri == "" {
		if err := si.downloadMedia(si.VideoUri, "video", []string{}); err != nil {
			return err
		}

		if err := os.Rename(WorkDir+"/video.mp4", DownloadDir+"/"+title+".mp4"); err != nil {
			return err
		}
	} else {
		if err := si.downloadMedia(si.VideoUri, "video", []string{"-bsf:a", "aac_adtstasc"}); err != nil {
			return err
		}
		if err := si.downloadMedia(si.AudioUri, "audio", []string{}); err != nil {
			return err
		}
		if err := si.mergeVideoAndAudio(title); err != nil {
			return err
		}
		if err := os.Remove(WorkDir + "/video.mp4"); err != nil {
			return err
		}
		if err := os.Remove(WorkDir + "/audio.mp4"); err != nil {
			return err
		}
	}

	return nil
}

func (si *StreamInfo) downloadMedia(uri string, mediaType string, params []string) error {
	var err error
	dlFiles := []string{}

	filepath := WorkDir + "/" + mediaType + ".m3u8"
	err = RetryDownload(uri, filepath)
	if err != nil {
		return err
	}

	dlFiles = append(dlFiles, filepath)

	defer func() {
		for _, file := range dlFiles {
			os.Remove(file)
		}
	}()

	uriPrefix, err := UriPrefix(uri)
	if err != nil {
		return err
	}

	bytes, err := ioutil.ReadFile(filepath)
	if err != nil {
		return err
	}

	lines := strings.Split(strings.Replace(string(bytes), "\r", "", -1), "\n")
	replacedFilepath := WorkDir + "/rw_" + mediaType + ".m3u8"
	file, err := os.Create(replacedFilepath)
	if err != nil {
		return err
	}

	defer func() {
		file.Close()
		os.Remove(replacedFilepath)
	}()

	writer := bufio.NewWriter(file)

	keyRe := regexp.MustCompile(`EXT-X-KEY.*URI="*([^"]+)"*`)
	uriRepRe := regexp.MustCompile(`URI="*[^"]+"*`)

	for _, line := range lines {
		// Download Key & replace URI on m3u8
		if keyMatches := keyRe.FindStringSubmatch(line); len(keyMatches) > 0 {
			keyFile := mediaType + ".key"
			err = RetryDownload(keyMatches[1], WorkDir+"/"+keyFile)
			if err != nil {
				return err
			}

			line = uriRepRe.ReplaceAllString(line, "URI=\""+keyFile+"\"")
			writer.Flush()

			dlFiles = append(dlFiles, WorkDir+"/"+keyFile)
		}

		// Download ts & replace path on m3u8
		if strings.Index(line, "#") == -1 && line != "" {
			tsFile, err := si.downloadTs(line, uriPrefix)
			if err != nil {
				return err
			}

			line = tsFile
			writer.Flush()

			dlFiles = append(dlFiles, WorkDir+"/"+tsFile)
		}

		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}
	writer.Flush()

	return si.mergeTs(replacedFilepath, mediaType+".mp4", params)
}

var httpRe = regexp.MustCompile(`^https?://`)

func (si *StreamInfo) downloadTs(tsPath string, uriPrefix string) (string, error) {
	var err error

	tsUri := tsPath
	tsFile := tsPath

	if httpRe.MatchString(tsUri) {
		if tsFile, err = UriBase(tsUri); err != nil {
			return "", err
		}
	} else {
		tsUri = uriPrefix + "/" + tsFile
	}

	downloadPath := WorkDir + "/" + tsFile
	fmt.Println("Download " + tsUri + " to " + downloadPath)
	return tsFile, RetryDownload(tsUri, downloadPath)
}

func (si *StreamInfo) mergeTs(m3u8Path string, outputFile string, params []string) error {
	params = append([]string{
		"-y",
		"-allowed_extensions", "ALL",
		"-i", m3u8Path,
		"-movflags", "faststart",
		"-c", "copy",
	}, params...)

	params = append(params, WorkDir+"/"+outputFile)

	return si.execFFMpeg(params)
}

func (si *StreamInfo) mergeVideoAndAudio(title string) error {
	params := []string{
		"-y",
		"-i", WorkDir + "/video.mp4",
		"-i", WorkDir + "/audio.mp4",
		"-c:v", "copy",
		"-c:a", "aac",
		DownloadDir + "/" + title + ".mp4",
	}

	return si.execFFMpeg(params)
}

func (si *StreamInfo) execFFMpeg(params []string) error {
	si.ctx, si.cancelFunc = context.WithCancel(context.Background())

	cmd := exec.CommandContext(si.ctx, FFMpegPath, params...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Wait()
}
