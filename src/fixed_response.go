// fixed_response.go — 固定返回 IM 主动回复（v34.0）
// 通过出站代理发送固定返回消息，复用已有的出站链路（含审计、检测）
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

// sendFixedReplyViaOutbound 构造蓝信消息格式，POST 到 localhost 出站端口
func (ip *InboundProxy) sendFixedReplyViaOutbound(senderID, text string) {
	outboundAddr := ip.cfg.OutboundListen
	if outboundAddr == "" {
		outboundAddr = ":18444"
	}
	// 蓝信消息格式：msgType 在外层，msgData.text.content 放内容
	msgPayload := map[string]interface{}{
		"userIdList": []string{senderID},
		"msgType":    "text",
		"msgData": map[string]interface{}{
			"text": map[string]string{
				"content": text,
			},
		},
	}
	body, _ := json.Marshal(msgPayload)
	// 蓝信 API 用 URL 参数传 app_token（不是 Authorization header）
	token, err := ip.getLanxinAppToken()
	if err != nil {
		log.Printf("[固定返回] 获取蓝信 token 失败: %v", err)
		return
	}
	targetURL := "http://127.0.0.1" + outboundAddr + "/v1/bot/messages/create?app_token=" + url.QueryEscape(token)
	req, err := http.NewRequest("POST", targetURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[固定返回] 构造出站请求失败: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[固定返回] 出站请求失败: %v", err)
		return
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[固定返回] 出站回复完成 sender=%s status=%d resp=%s", senderID, resp.StatusCode, truncate(string(respBody), 200))
}

// getLanxinAppToken 获取蓝信 app_token
func (ip *InboundProxy) getLanxinAppToken() (string, error) {
	cfg := ip.cfg
	upstream := cfg.LanxinUpstream
	if upstream == "" {
		upstream = "https://apigw.lx.qianxin.com"
	}
	tokenURL := upstream + "/v1/apptoken/create?grant_type=client_credential&appid=" +
		url.QueryEscape(cfg.LanxinAppID) + "&secret=" + url.QueryEscape(cfg.LanxinAppSecret)
	resp, err := http.Get(tokenURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var tokenResp struct {
		ErrCode int `json:"errCode"`
		Data    struct {
			AppToken string `json:"appToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("解析token响应失败: %s", string(body))
	}
	if tokenResp.Data.AppToken == "" {
		return "", fmt.Errorf("token为空: %s", string(body))
	}
	return tokenResp.Data.AppToken, nil
}
