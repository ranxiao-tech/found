	package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/beewit/beekit/redis"
	"github.com/beewit/found/global"
	"github.com/sclevine/agouti"
	"github.com/sclevine/agouti/api"
	"regexp"
	"github.com/beewit/beekit/utils"
	"github.com/beewit/beekit/utils/convert"
)

const (
	QQ_SPIDER        = "QQ_SPIDER"
	QQ_SPIDER_DONE   = "QQ_SPIDER_DONE"
	QQ_SPIDER_FAILED = "QQ_SPIDER_FAILED"
	QQ_SPIDER_DATA   = "QQ_SPIDER_DATA"
)

func addQueue(value string) {
	if !checkDoneQueue(value) && !checkFailedQueue(value) && !checkQueue(value) {
		global.RD.SetSETString(QQ_SPIDER, value)
	}
}
func getQueue() string {
	v, _ := global.RD.GetSETRandStringRm(QQ_SPIDER)
	return v
}
func checkQueue(value string) bool {
	x, _ := global.RD.CheckSETString(QQ_SPIDER, value)
	return x > 0
}

func addDoneQueue(value string) {
	global.RD.SetSETString(QQ_SPIDER_DONE, value)
}
func getDoneQueue(value string) string {
	v, _ := global.RD.GetSETRandStringRm(QQ_SPIDER_DONE)
	return v
}
func checkDoneQueue(value string) bool {
	x, _ := global.RD.CheckSETString(QQ_SPIDER_DONE, value)
	return x > 0
}

func addFailedQueue(value string) {
	global.RD.SetSETString(QQ_SPIDER_FAILED, value)
}
func getFailedQueue(value string) string {
	v, _ := global.RD.GetSETRandStringRm(QQ_SPIDER_FAILED)
	return v
}
func checkFailedQueue(value string) bool {
	x, _ := global.RD.CheckSETString(QQ_SPIDER_FAILED, value)
	return x > 0
}

func addDataQueue(value map[string]interface{}) {
	b, err := json.Marshal(value)
	if err != nil {
		global.Log.Info(err.Error())
		return
	}
	x, err := global.RD.SetSETString(QQ_SPIDER_DATA, string(b))
	if err != nil {
		global.Log.Info(err.Error())
	}
	global.Log.Info("添加数据到Redis结果%v", x)
}

func getDataQueue() (string, error) {
	return global.RD.GetSETRandStringRm(QQ_SPIDER_DATA)
}

func getData() map[string]interface{} {
	d, err := getDataQueue()
	if err != nil {
		global.Log.Warning("无数据", err.Error())
		return nil
	}
	if d != "" {
		m := map[string]interface{}{}
		err = json.Unmarshal([]byte(d), &m)
		if err == nil {
			return m
		}
	}
	return nil
}

func saveData() {
	for {
		global.Log.Info("执行QQ数据保存服务")
		m := getData()
		if m != nil {
			qqBase := map[string]interface{}{}
			qqBaseId  := utils.ID()
			qqBase["id"] = qqBaseId
			qqBase["qq"] = m["qq"]
			qqBase["nickname"] = m["nice_name"]
			qqBase["head_img"] = m["head_img"]
			qqBase["url"] = m["url"]

			if m["info"] != nil {
				info, err := convert.Obj2Map(m["info"])
				if err != nil {
					global.Log.Error("info 转换失败:", err.Error())
				} else {
					qqBase["gender"] = info["sex"]
					qqBase["age"] = info["age"]
					qqBase["constell"] = info["astro"]
					qqBase["birthday"] = info["birthday"]
					qqBase["place"] = info["live_address"]
					qqBase["marriage"] = info["marriage"]
					qqBase["blood_type"] = info["blood"]
					qqBase["hometown"] = info["hometown_address"]
					qqBase["profession"] = info["career"]
					qqBase["company"] = info["company"]
					qqBase["company_address"] = info["company_caddress"]
					qqBase["detail_address"] = info["caddress"]
				}
			}

			qqBase["ct_time"] = utils.CurrentTime()
			qqBase["ut_time"] = utils.CurrentTime()
			_, err := global.DB.InsertMap("qq_user_base_info", qqBase)
			if err != nil {
				global.Log.Error("qq_user_base_info 数据保存失败:", err.Error())
			} else {
				//QQ动态
				if m["tell"] != nil {
					sayMapList := []map[string]interface{}{}
					tell, err := convert.Obj2ListMap(m["tell"])
					if err != nil {
						global.Log.Error("tell QQ 动态 转换失败:", err.Error())
					} else {
						for i := 0; i < len(tell); i++ {
							sayMap := map[string]interface{}{}
							comments := ""
							if tell[i]["tell_comments"] != nil {
								b, err := json.Marshal(tell[i]["tell_comments"])
								if err != nil {
									global.Log.Error("tell_comments 评论 Json转换失败:", err.Error())
								} else {
									comments = string(b)
								}
							}
							sayMap["id"] = utils.ID()
							sayMap["qq_base_id"] = qqBaseId
							sayMap["say"] = tell[i]["tell_text"]
							sayMap["content"] = tell[i]["tell_summary"]
							sayMap["phone"] = tell[i]["tell_phone"]
							sayMap["browse"] = tell[i]["tell_browse"]
							sayMap["comments"] = comments
							sayMap["say_date"] = tell[i]["tell_date"]
							sayMap["ct_time"] = utils.CurrentTime()
							sayMapList = append(sayMapList, sayMap)
						}
					}
					if sayMapList != nil && len(sayMapList) > 0 {
						_, err = global.DB.InsertMapList("qq_say", sayMapList)
						if err != nil {
							global.Log.Error("qq_say 数据保存失败:", err.Error())
						}
					}
				}
			}
		}
		time.Sleep(time.Second * 10)

	}
}

func main() {
	go saveData()
	driver := agouti.ChromeDriver(agouti.ChromeOptions("args", []string{
		"--start-maximized",
		"--disable-infobars",
		"--app=https://i.qq.com/?rd=1",
		"--webkit-text-size-adjust"}))
	driver.Start()
	var err error
	global.Page.Page, err = driver.NewPage()
	if err != nil {
		global.Log.Info(err.Error())
	} else {
		flog, ce := SetCookieLogin("qqzone" + global.QZONEUName)
		if ce != nil {
			global.Log.Info(ce.Error())
			return
		}
		if flog {
			global.Page.Navigate("https://user.qzone.qq.com/" + global.QZONEUName)
			time.Sleep(time.Second * 3)
			src, ei := global.Page.Find(".head-avatar img").Attribute("src")
			if ei != nil {
				global.Log.Info(ei.Error())
			}
			if src != "" {
				global.Log.Info("头像", src)
			} else {
				flog = false
			}
		}
		if !flog {
			global.Page.Navigate("https://i.qq.com/?rd=1")
			time.Sleep(time.Second * 3)
			iframe, ee := global.Page.Find("#login_frame").Elements()
			if ee != nil {
				global.Log.Info(ee.Error())
			}
			e2 := global.Page.SwitchToRootFrameByName(iframe[0])
			if e2 != nil {
				global.Log.Info(e2.Error())
				return
			}
			text, e3 := global.Page.Find("#switcher_plogin").Text()
			if e3 != nil {
				global.Log.Info(e3.Error())
				return
			}
			global.Log.Info("登陆按钮", text)
			e4 := global.Page.Find("#switcher_plogin").Click()
			if e4 != nil {
				global.Log.Info("登陆失败", e4.Error())
				return
			}
			global.Page.FindByID("u").Fill(global.QZONEUName)
			time.Sleep(time.Second * 1)
			global.Page.FindByID("p").Fill(global.QZONEPwd)
			time.Sleep(time.Second * 2)
			global.Page.FindByID("login_button").Click()
			time.Sleep(time.Second * 2)
			global.Page.SwitchToParentFrame()
			time.Sleep(time.Second * 3)

			iframe, ef := global.Page.Find("#login_frame").Elements()
			if ef != nil {
				global.Log.Info(ef.Error())
			}
			if iframe != nil && len(iframe) > 0 {
				eef := global.Page.SwitchToRootFrameByName(iframe[0])
				if eef != nil {
					global.Log.Info(eef.Error())
				}
			}
			for {
				//是否有验证码
				html, _ := global.Page.HTML()
				if strings.Contains(html, "请您输入下图中的验证码") {
					global.Log.Info("等待输入验证码")
					continue
				}
				if strings.Contains(html, "QQ手机版授权") {
					global.Log.Info("等待QQ手机版授权")
					continue
				}
				if strings.Contains(html, "安全登录") {
					global.Log.Info("等待手工登陆")
					continue
				}
				time.Sleep(time.Second * 1)
				break
			}

			c, err5 := global.Page.GetCookies()
			if err5 != nil {
				global.Log.Info("登陆失败", e4.Error())
				return
			}
			cookieJson, _ := json.Marshal(c)
			//global.Log.Info("cookie", string(cookieJson[:]))
			redis.Cache.SetString("qqzone"+global.QZONEUName, string(cookieJson[:]))

		}
		////数据
		count := 0
		////初步获取QQ号码
		getHome(global.Page, count)
		//global.Page.SwitchToParentFrame()
		//获取他人个人资料，个人动态，群等
		getQQ(global.Page, global.QZONEUName)

	}
}

func getQQ(page *utils.AgoutiPage, qq string) {
	addDoneQueue(qq)
	global.Log.Info("一、爬取的QQ", qq)
	url := "https://user.qzone.qq.com/" + qq
	global.Page.Navigate(url + "/311")
	//global.Page.Navigate(url)
	global.Log.Info("二、爬取的QQ《", qq, "》已进入说说页面跳转完成", qq)
	time.Sleep(time.Second * 3)
	m := map[string]interface{}{}
	html, _ := global.Page.HTML()
	global.Log.Info("三、爬取的QQ《", qq, "》解析页面结果量", len(html))
	if !strings.Contains(html, "申请访问") && !strings.Contains(html, "不符合互联网相关安全规范") && !strings.Contains(html, "对方未开通空间") && !strings.Contains(html, "暂不支持非好友访问") && !strings.Contains(html, "您访问的页面找不回来了") {
		nice_name, err := global.Page.Find(".head-detail .user-name").Text()
		if err != nil {
			global.Log.Error(err.Error())
		}
		m["nice_name"] = nice_name
		m["qq"] = qq
		m["head_img"], _ = global.Page.FindByID("QM_OwnerInfo_Icon").Attribute("src")
		m["url"] = url
		//个人动态
		tell := getTell(page, qq)
		m["tell"] = tell
		//个人资料
		info := getInfo(page, qq)
		m["info"] = info
		addDataQueue(m)
	}
	newQQ := getQueue()
	if newQQ != "" {
		getQQ(page, newQQ)
	} else {
		global.Log.Warning("                                                                                               ")
		global.Log.Warning("========================================                 =====================================")
		global.Log.Warning("=======================================   无可爬取的QQ号了   =====================================")
		global.Log.Warning("========================================                 =====================================")
		global.Log.Warning("                                                                                               ")
		global.Log.Warning("                                                                                               ")
		global.Log.Warning("                                    》》》》》3分钟后重启《《《《《                                 ")
		global.Log.Warning("                                                                                               ")
		time.Sleep(time.Minute * 3)
		////数据
		count := 0
		getHome(global.Page, count)
		getQQ(global.Page, global.QZONEUName)
	}
}

func SwitchFrame(page *utils.AgoutiPage, frameSeletor string, f func()) {
	iframe, ee := global.Page.Find(frameSeletor).Elements()
	if ee != nil {
		global.Log.Info(ee.Error())
	}
	if len(iframe) > 0 {
		ee2 := global.Page.SwitchToRootFrameByName(iframe[0])
		if ee2 != nil {
			global.Log.Info(ee.Error())
		}
		f()
		global.Page.SwitchToParentFrame()
	} else {
		global.Log.Info("******     iframe   切换失败")
	}
}

//个人动态
func getTell(page *utils.AgoutiPage, qq string) []map[string]interface{} {

	global.Log.Info("四、解析QQ《", qq, "》说说")
	//global.Page.Navigate("https://user.qzone.qq.com/" + qq + "/311")
	//time.Sleep(time.Second * 3)

	iframe, ee := global.Page.Find(".app_canvas_frame").Elements()
	if ee != nil {
		global.Log.Info(ee.Error())
	}
	if len(iframe) > 0 {
		ee2 := global.Page.SwitchToRootFrameByName(iframe[0])
		if ee2 != nil {
			global.Log.Info(ee.Error())
		}
	}
	time.Sleep(time.Second * 3)
	saveQQ(page)
	ms := []map[string]interface{}{}
	eles, _ := global.Page.Find("#host_home_feeds li").Elements()
	if eles != nil {
		for i := range eles {
			m := map[string]interface{}{}
			//说说
			e, _ := eles[i].GetElement(api.Selector{"css selector", ".f-single-content .f-info"})
			if e != nil {
				m["tell_text"], _ = e.GetText()
			}
			//图片或转发或第三方链接
			e, _ = eles[i].GetElement(api.Selector{"css selector", ".f-single-content .qz_summary"})
			if e != nil {
				m["tell_summary"], _ = e.GetText()
				//手机型号
				e, _ = eles[i].GetElement(api.Selector{"css selector", ".phone-style"})
				if e != nil {
					m["tell_phone"], _ = e.GetText()
				}
			}
			//浏览量
			e, _ = eles[i].GetElement(api.Selector{"css selector", "a.qz_feed_plugin"})
			if e != nil {
				m["tell_browse"], _ = e.GetText()
			}
			//点赞
			es, _ := eles[i].GetElements(api.Selector{"css selector", ".f-like-list a.q_namecard"})
			if es != nil {
				var us []map[string]string
				for j := range es {
					u, _ := es[j].GetText()
					h, _ := es[j].GetAttribute("href")
					us = append(us, map[string]string{u: h})
				}
				if len(us) > 0 {
					m["tell_like_users"] = us
				}
			}
			//评论
			es, _ = eles[i].GetElements(api.Selector{"css selector", ".comments-content"})
			if es != nil {
				var cs []string
				for j := range es {
					u, _ := es[j].GetText()
					cs = append(cs, u)
				}
				if len(cs) > 0 {
					m["tell_comments"] = cs
				}
			}
			ms = append(ms, m)
		}
	} else {
		html, _ := global.Page.Find("#msgList").Text()
		global.Log.Info("解析html数据量%v", len(html))

		eles, _ := global.Page.Find("#msgList").All(".feed").Elements()
		if eles != nil {
			for i := range eles {
				m := map[string]interface{}{}
				//说说
				e, _ := eles[i].GetElement(api.Selector{"css selector", ".bd pre"})
				if e != nil {
					m["tell_text"], _ = e.GetText()
				}
				//图片或转发或第三方链接
				e, _ = eles[i].GetElement(api.Selector{"css selector", ".rt_content"})
				if e != nil {
					m["tell_summary"], _ = e.GetText()
					//手机型号
					e, _ = eles[i].GetElement(api.Selector{"css selector", ".custom-tail"})
					if e != nil {
						m["tell_phone"], _ = e.GetText()
					}
				}
				//发表时间
				e, _ = eles[i].GetElement(api.Selector{"css selector", ".bgr3>.ft .info a"})
				if e != nil {
					m["tell_date"], _ = e.GetText()
				}
				//点赞的人
				es, _ := eles[i].GetElements(api.Selector{"css selector", ".feed_like a"})
				if es != nil {
					var us []map[string]string
					for j := range es {
						u, _ := es[j].GetText()
						h, _ := es[j].GetAttribute("href")
						us = append(us, map[string]string{u: h})
					}
					if len(us) > 0 {
						m["tell_like_users"] = us
					}
				}
				//浏览量
				e, _ = eles[i].GetElement(api.Selector{"css selector", "a.qz_feed_plugin"})
				if e != nil {
					m["tell_browse"], _ = e.GetText()
				}
				//评论
				es, _ = eles[i].GetElements(api.Selector{"css selector", ".comments_content"})
				if es != nil {
					var cs []string
					for j := range es {
						u, _ := es[j].GetText()
						cs = append(cs, u)
					}
					if len(cs) > 0 {
						m["tell_comments"] = cs
					}
				}
				ms = append(ms, m)

			}
		}
	}
	global.Page.SwitchToParentFrame()
	return ms
}

//个人资料
func getInfo(page *utils.AgoutiPage, qq string) map[string]string {
	global.Log.Info("四、解析QQ《", qq, "》个人资料")
	global.Page.Navigate("https://user.qzone.qq.com/" + qq + "/1")
	time.Sleep(time.Second * 3)
	iframe, ee := global.Page.Find(".app_canvas_frame").Elements()
	if ee != nil {
		global.Log.Info(ee.Error())
	}
	if len(iframe) <= 0 {
		return nil
	}
	ee2 := global.Page.SwitchToRootFrameByName(iframe[0])
	if ee2 != nil {
		global.Log.Info(ee2.Error())
	}

	text, ee3 := global.Page.FindByID("info_preview").Text()
	if ee3 != nil {
		global.Log.Info(ee3.Error())
		global.Page.FindByID("info_tab").Click()
	}
	if text == "" {
		global.Page.FindByID("info_tab").Click()
	}
	time.Sleep(time.Second * 2)

	m := map[string]string{}
	m["sex"], _ = global.Page.FindByID("sex").Text()
	m["age"], _ = global.Page.FindByID("age").Text()
	m["birthday"], _ = global.Page.FindByID("birthday").Text()
	m["astro"], _ = global.Page.FindByID("astro").Text()
	m["live_address"], _ = global.Page.FindByID("live_address").Text()
	m["marriage"], _ = global.Page.FindByID("marriage").Text()
	m["blood"], _ = global.Page.FindByID("blood").Text()
	m["hometown_address"], _ = global.Page.FindByID("hometown_address").Text()
	m["career"], _ = global.Page.FindByID("career").Text()
	m["company"], _ = global.Page.FindByID("company").Text()
	m["company_caddress"], _ = global.Page.FindByID("company_caddress").Text()
	m["caddress"], _ = global.Page.FindByID("caddress").Text()
	global.Page.SwitchToParentFrame()
	return m
}

func saveQQ(page *utils.AgoutiPage) {
	html, _ := global.Page.HTML()
	reg := regexp.MustCompile("http(s)?://user.qzone.qq.com/\\d+")
	strs := reg.FindAllString(html, -1)
	for i := 0; i < len(strs); i++ {
		s := strings.Replace(strs[i], "http://user.qzone.qq.com/", "", -1)
		s = strings.Replace(s, "https://user.qzone.qq.com/", "", -1)
		if s != "" {
			addQueue(s)
		}
	}
}

//个人主页获取QQ信息
func getHome(page *utils.AgoutiPage, count int) []map[string]string {
	saveQQ(page)

	list, e6 := global.Page.Find("#feed_friend_list").All(".f-single").Elements()
	if e6 != nil {
		global.Log.Info("获取好友数据失败", e6.Error())
		return nil
	}
	global.Log.Info("总数量", len(list))
	var s string
	var e7 error
	var ele *api.Element
	for i := range list {

		global.Log.Info("---------------------------------------------------------\r\n")

		s, e7 = list[i].GetAttribute("id")
		if e7 != nil {
			global.Log.Info("错误：", e7.Error())
		}
		global.Log.Info("id：", s)
		ele, e7 = list[i].GetElement(api.Selector{"css selector", ".user-pto img"})
		if e7 != nil {
			global.Log.Info("错误：", e7.Error())
		}
		s, e7 = ele.GetAttribute("src")
		global.Log.Info("头像：", s)

		ele, _ = list[i].GetElement(api.Selector{"css selector", ".user-pto a"})
		if e7 != nil {
			global.Log.Info("错误：", e7.Error())
		}
		s, e7 = ele.GetAttribute("href")
		global.Log.Info("空间链接：", s)

		ele, _ = list[i].GetElement(api.Selector{"css selector", ".f-single-content"})
		if e7 != nil {
			global.Log.Info("错误：", e7.Error())
		}
		s, e7 = ele.GetText()
		global.Log.Info("发表内容：", s)

		ele, _ = list[i].GetElement(api.Selector{"css selector", ".qz_feed_plugin"})
		if e7 != nil {
			global.Log.Info("错误：", e7.Error())
		}
		s, e7 = ele.GetText()
		global.Log.Info("浏览量：", s)

		ele, _ = list[i].GetElement(api.Selector{"css selector", ".comments-list"})
		if e7 != nil {
			global.Log.Info("错误：", e7.Error())
		}
		s, e7 = ele.GetText()
		global.Log.Info("评论：", s)

		ele, _ = list[i].GetElement(api.Selector{"css selector", ".user-list"})
		if e7 != nil {
			global.Log.Info("错误：", e7.Error())
		}
		s, e7 = ele.GetText()
		global.Log.Info("点赞：", s)

		global.Log.Info("---------------------------------------------------------\r\n")
	}
	global.Page.RunScript("document.documentElement.scrollTop=document.body.clientHeight;", nil, nil)
	time.Sleep(time.Second * 3)
	count++
	//分页次数
	if count < 3 {
		getHome(page, count)
	}
	return nil
}

func SetCookieLogin(key string) (bool, error) {
	rc := redis.Cache
	cookieRd, _ := rc.GetString(key)
	if cookieRd == "" {
		return false, nil
	}
	var cks = []*http.Cookie{}
	err := json.Unmarshal([]byte(cookieRd), &cks)
	if err != nil {
		return false, err
	}
	for i := range cks {
		cc := cks[i]
		global.Page.SetCookie(cc)
	}
	return true, nil
}
