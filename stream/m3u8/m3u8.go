package m3u8

import (
	"fmt"
	"github.com/donething/live-dl-go/hanlders"
	"github.com/donething/live-dl-go/m3u8decoder"
	"github.com/donething/live-dl-go/stream/entity"
)

// Stream m3u8 直播流
type Stream struct {
	*entity.Stream
}

// NewM3u8Stream 创建 Stream 的实例
func NewM3u8Stream(title, streamUrl string, headers map[string]string, path string,
	fileSizeThreshold int, handler hanlders.IHandler) entity.IStream {
	return &Stream{
		Stream: &entity.Stream{
			Title:             title,
			LiveStreamUrl:     streamUrl,
			Headers:           headers,
			ChSegUrl:          make(chan string),
			Path:              path,
			FileSizeThreshold: fileSizeThreshold,
			Handler:           handler,
			ChErr:             make(chan error),
			ChRestart:         make(chan bool),
		},
	}
}

func (s *Stream) Start() error {
	err := s.PrepareCapture()
	if err != nil {
		return fmt.Errorf("准备录制m3u8流时出错：%w", err)
	}

	go s.sendSeq()

	return nil
}

func (s *Stream) sendSeq() {
	for {
		// 解码 m3u8 视频列表
		m := m3u8decoder.New()
		err := m.Decode(s.LiveStreamUrl, s.Headers)
		if err != nil {
			s.Stream.ChErr <- fmt.Errorf("解码 m3u8 文件出错：%w", err)
			return
		}

		// 如果没有获取到新的切片，表示直播结束
		if len(m.Segments) == 0 {
			close(s.ChSegUrl)
			break
		}

		// 发送切片的 URL 下载
		for _, seg := range m.Segments {
			select {
			case s.ChSegUrl <- seg.URL:
			default:
				// 缓冲区已满，继续等待
			}
		}
	}
	// 	不用返回 nil 给 s.chErr，以免返回后主函数继续执行下一步，而此时直播流还没有下载成功（只是发送URL成功）
}

func (s *Stream) GetChErr() chan error {
	return s.ChErr
}

func (s *Stream) GetChRestart() chan bool {
	return s.ChRestart
}

func (s *Stream) Reset(title, streamUrl string, headers map[string]string, path string,
	fileSizeThreshold int, hanlder hanlders.IHandler) {
	s.ChErr = make(chan error)
	s.ChRestart = make(chan bool)
	s.ChSegUrl = make(chan string)

	s.Title = title
	s.LiveStreamUrl = streamUrl
	s.Headers = headers
	s.Path = path
	s.FileSizeThreshold = fileSizeThreshold
	s.Handler = hanlder
}