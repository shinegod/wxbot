package wordcloud

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/imroc/req/v3"
	"github.com/tidwall/gjson"

	"github.com/yqchilde/wxbot/engine/control"
	"github.com/yqchilde/wxbot/engine/pkg/log"
	"github.com/yqchilde/wxbot/engine/robot"
)

func init() {
	engine := control.Register("wordcloud", &control.Options{
		Alias: "词云",
		Help: "输入 {热词} => 获取当前聊天室热词，默认当前聊天室Top30条\n" +
			"输入 {热词 top 10} => 获取当前聊天室热词前10条\n" +
			"输入 {热词 id xxx} => 获取指定聊天室热词\n" +
			"输入 {热词 id xxx top 10} => 获取指定聊天室热词前10条\n",
		DataFolder: "wordcloud",
	})

	cachePath := engine.GetDataFolder() + "/cache"
	_ = os.MkdirAll(cachePath, 0755)

	engine.OnRegex(`^热词(?:\s+id\s+(\S+))?(?:\s+top\s+(\d+))?$|^热词\s+top\s+(\d+)$`).SetBlock(true).Handle(func(ctx *robot.Ctx) {
		id := ctx.State["regex_matched"].([]string)[1]
		top, _ := strconv.ParseInt(ctx.State["regex_matched"].([]string)[2], 10, 64)

		// todo 5分钟允许拿一次数据就行了，有空在搞

		if id == "" && top == 0 { // 获取当前群，top默认30
			id = ctx.Event.FromUniqueID
			top = 30
		} else if id != "" && top == 0 { // 获取指定群，top默认30
			top = 30
		} else if id == "" && top != 0 { // 获取当前群，top指定
			id = ctx.Event.FromUniqueID
		} else if id != "" && top != 0 { // 获取指定群，top指定
			// do nothing
		}

		// 获取历史记录-文本消息
		record, err := ctx.GetHistoryByWxId(id)
		if err != nil {
			log.Errorf("获取[%s]热词失败: %v", id, err)
			ctx.ReplyText("获取热词失败")
			return
		}

		// 整理文本消息
		var words string
		for _, msg := range record {
			// 剔除消息中的表情
			for _, emoji := range robot.EmojiSymbol {
				msg.Content = strings.ReplaceAll(msg.Content, emoji, "")
			}
			// 剔除消息中的艾特
			if strings.HasPrefix(msg.Content, "@") {
				msg.Content = msg.Content[strings.Index(msg.Content, " "):]
			}
			words += msg.Content + " "
		}

		// 获取热词图
		resp := req.C().Post("https://bot.yqqy.top/api/wordcloud").SetBody(map[string]interface{}{"words": words, "count": top}).Do()
		if resp.GetStatusCode() != 200 {
			log.Errorf("获取[%s]热词失败: %v", id, err)
			ctx.ReplyText("获取热词失败")
			return
		}
		if gjson.Get(resp.String(), "code").Int() != 200 {
			log.Errorf("获取[%s]热词失败: %v", id, err)
			ctx.ReplyText("获取热词失败")
			return
		}

		// 保存图片
		imgB64 := gjson.Get(resp.String(), "data.image").String()
		filename := fmt.Sprintf("%s/%s_%s.png", engine.GetDataFolder()+"/cache", ctx.Event.FromUniqueID, time.Now().Local().Format("20060102"))
		if err := base64ToImage(filename, imgB64); err != nil {
			log.Errorf("保存图片失败: %v", err)
			ctx.ReplyText("获取热词失败")
			return
		}

		// 上传图片
		resp = req.C().Post("https://bot.yqqy.top/api/uploadImg").SetFile("file", filename).Do()
		if resp.GetStatusCode() != 200 {
			log.Errorf("上传图片失败: %v", err)
			ctx.ReplyText("获取热词失败")
			return
		}
		if gjson.Get(resp.String(), "code").Int() != 200 {
			log.Errorf("上传图片失败: %v", err)
			ctx.ReplyText("获取热词失败")
			return
		}
		ctx.ReplyImage(gjson.Get(resp.String(), "data").String())
	})
}

func base64ToImage(src string, base64Str string) error {
	imgByte, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return err
	}
	file, err := os.Create(src)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, bytes.NewReader(imgByte))
	if err != nil {
		return err
	}
	return nil
}