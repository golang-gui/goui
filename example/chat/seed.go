package main

const (
	today     = "今天"
	yesterday = "昨天"
)

// newStore returns the fixed sample conversations. Every name and message is
// fictional; nothing is loaded from disk or the network.
func newStore() *store {
	return &store{contacts: []*contact{
		{
			ID: "lin", Name: "林薇", Color: 0, Online: true,
			Messages: []message{
				{Sender: "林薇", Text: "早，周五的评审材料我放到共享盘了", Day: yesterday, Time: "18:42"},
				{Text: "收到，今晚看完给你反馈", Day: yesterday, Time: "18:45"},
				{Sender: "林薇", Text: "嗨，最近在忙什么？", Day: today, Time: "10:31"},
				{Text: "在写一个 Go 的跨平台 GUI 框架", Day: today, Time: "10:32"},
				{Text: "刚做完一个系统监控示例，曲线和数字都是动的", Day: today, Time: "10:32"},
				{Sender: "林薇", Text: "听起来很酷！是原生控件还是自己画的？", Day: today, Time: "10:34"},
				{Text: "自己画的。窗口、事件和文字排版走平台层，控件、布局和样式都在 Go 里实现。", Day: today, Time: "10:35"},
				{Sender: "林薇", Text: "那三个平台看起来能一致吗", Day: today, Time: "10:36"},
			},
			replies: []reply{
				{text: "下次给我演示一下吧 😄"},
				{text: "我们组正好在找能打包成单个可执行文件的方案"},
				{text: "好的，周五见！"},
			},
		},
		{
			ID: "team", Name: "设计小组", Color: 4, Group: true, Online: true, Unread: 3,
			Members: map[string]int{"周宁": 2, "陈一鸣": 3, "Alex": 1},
			Messages: []message{
				{Sender: "周宁", Text: "新版配色稿传上来了，暗色模式也一起做了", Day: today, Time: "09:12"},
				{Sender: "陈一鸣", Text: "卡片圆角改成 10 会不会更柔和一点？", Day: today, Time: "09:20"},
				{Text: "可以，我先在示例里试一下", Day: today, Time: "09:24"},
				{Sender: "Alex", Text: "Looks great. Can we keep the accent to one color?", Day: today, Time: "09:31"},
				{Sender: "周宁", Text: "同意，强调色只留一个", Day: today, Time: "09:33"},
				{Sender: "陈一鸣", Text: "下午三点过一下交互细节", Day: today, Time: "09:40"},
			},
			replies: []reply{
				{sender: "周宁", text: "收到 👍"},
				{sender: "Alex", text: "Sounds good, see you at 3."},
				{sender: "陈一鸣", text: "我把会议室订好了"},
			},
		},
		{
			ID: "tanaka", Name: "田中陽", Color: 1, Seen: "1 小时前在线", Unread: 1,
			Messages: []message{
				{Text: "东京的发布会顺利吗？", Day: yesterday, Time: "21:03"},
				{Sender: "田中陽", Text: "很顺利，现场演示一次就过了", Day: yesterday, Time: "21:15"},
				{Sender: "田中陽", Text: "照片晚点发你", Day: today, Time: "08:47"},
			},
			replies: []reply{{text: "See you later!"}},
		},
		{
			ID: "ming", Name: "小明", Color: 3, Seen: "昨天在线",
			Messages: []message{
				{Sender: "小明", Text: "周末去爬山吗？", Day: yesterday, Time: "12:10"},
				{Text: "天气好就去", Day: yesterday, Time: "12:30"},
			},
			replies: []reply{{text: "那就说定了，早上七点集合"}},
		},
		{
			ID: "release", Name: "发布值班", Color: 2, Group: true,
			Members: map[string]int{"CI 机器人": 2, "王磊": 5},
			Messages: []message{
				{Sender: "CI 机器人", Text: "构建 #1284 通过：linux/amd64、windows/amd64、darwin/arm64", Day: "周一", Time: "17:02"},
				{Sender: "王磊", Text: "签名和公证也完成了", Day: "周一", Time: "17:20"},
			},
			replies: []reply{{sender: "王磊", text: "收到，候选版本已冻结"}},
		},
		{
			ID: "su", Name: "苏晴", Color: 5, Seen: "3 天前在线",
			Messages: []message{
				{Sender: "苏晴", Text: "上次推荐的书很好看，谢谢！", Day: "周日", Time: "20:18"},
			},
			replies: []reply{{text: "下次再给我推荐几本 📚"}},
		},
	}}
}
