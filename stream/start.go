package stream

import (
	"fmt"
	"github.com/donething/live-dl-go/comm/logger"
	"github.com/donething/live-dl-go/hanlders"
	"github.com/donething/live-dl-go/sites/entity"
	"github.com/donething/live-dl-go/sites/plats"
	streamentity "github.com/donething/live-dl-go/stream/entity"
	"github.com/donething/live-dl-go/stream/flv"
	"github.com/donething/live-dl-go/stream/m3u8"
	"github.com/donething/utils-go/dotext"
	"strings"
	"sync"
	"time"
)

const (
	// 获取主播信息失败后，重试的次数
	maxRetry = 3
)

// StartAnchor 开始录制直播流
//
// 参数为 正在录制表、直播流（Flv、M3u8）、主播信息、临时文件存储路径、单视频大小、视频处理器
//
// 当 stream 为 nil 时，将根据直播流地址自动生成
func StartAnchor(capturing *sync.Map, stream streamentity.IStream, anchor entity.Anchor, path string,
	fileSizeThreshold int64, handler hanlders.IHandler) error {
	// 开始录制该主播的时间
	start := dotext.FormatDate(time.Now(), "20060102")

	anchorSite, err := plats.GenAnchor(&anchor)
	if err != nil {
		return err
	}

	// 	获取主播信息
	info, err := tryGetAnchorInfo(anchorSite, maxRetry)
	if err != nil {
		return err
	}

	// 是否正在录播的键
	key := GenCapturingKey(&anchor)

	if !info.IsLive {
		logger.Info.Printf("😴【%s】没有在播(%+v)\n", info.Name, anchor)
		capturing.Delete(key)
		return nil
	}

	// 判断此次是否需要录制视频
	// 存在表示正在录制且此次不用换新文件存储，不重复录制，返回
	if s, exists := capturing.Load(key); exists {
		var bytes = ""
		if ss, ok := s.(streamentity.IStream); ok {
			bytes = dotext.BytesHumanReadable(ss.GetStream().CurBytes.GetBytes())
		}

		logger.Info.Printf("😊【%s】正在录制(%+v)，当前文件已写入 %s/%s\n", info.Name, anchor,
			bytes, dotext.BytesHumanReadable(fileSizeThreshold))
		return nil
	}

	// 需要开始录制

	// 生成标题
	// 平台对应的网站名
	title := hanlders.GenTgCaption(info.Name, anchorSite.GetPlatName(), start, info.Title)
	headers := anchorSite.GetStreamHeaders()

	// 如果没有指定直播流的类型，就自动匹配
	if stream == nil {
		if strings.Contains(strings.ToLower(info.StreamUrl), ".flv") {
			stream = flv.NewStream(title, info.StreamUrl, headers, path, fileSizeThreshold, handler)
		} else if strings.Contains(strings.ToLower(info.StreamUrl), ".m3u8") {
			stream = m3u8.NewStream(title, info.StreamUrl, headers, path, fileSizeThreshold, handler)
		} else {
			return fmt.Errorf("没有匹配到直播流的类型：%s", info.StreamUrl)
		}
	}

	// 开始录制直播流
	logger.Info.Printf("😙【%s】开始录制直播(%+v)\n", info.Name, anchor)

	// 记录正在录制的标识
	capturing.Store(key, stream)

	err = stream.Capture()
	// 当录制出错时，要判断出错情况：在获取直播流出错时，先判断主播此时是否在播，主播且出错才是真正的录制错误
	if err != nil {
		infoCheck, err := tryGetAnchorInfo(anchorSite, maxRetry)
		if err != nil {
			return err
		}

		if infoCheck.IsLive {
			return err
		}
	}

	// 已下播，结束录制
	logger.Info.Printf("😶【%s】已中断直播(%+v)，停止录制\n", info.Name, anchor)
	capturing.Delete(key)

	return nil
}

// GenCapturingKey 正在录制的主播的键，避免重复录制，格式如 "<平台>_<主播ID>"，如 "bili_12345"
func GenCapturingKey(anchor *entity.Anchor) string {
	return fmt.Sprintf("%s_%s", anchor.Plat, anchor.ID)
}

// 获取主播信息，可指定失败后的重试次数
func tryGetAnchorInfo(anchorSite entity.IAnchor, retry int) (*entity.AnchorInfo, error) {
	fail := 0
	var info *entity.AnchorInfo
	var err error

	for {
		info, err = anchorSite.GetAnchorInfo()
		if err != nil {
			// 重试
			if fail < retry {
				fail++
				time.Sleep(1 * time.Second)
				continue
			}

			return nil, err
		}

		// 获取成功
		return info, nil
	}
}
