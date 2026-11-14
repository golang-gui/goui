package main

// 测试程序与go generate复用同一份主题源码；运行时写入私有临时目录。
var themeSources = map[string]string{
	"themes/GouiXSettingsDark/gtk-3.0/gtk.css": "@define-color theme_selected_bg_color #00aa44;\n",
	"themes/GouiXSettingsRed/gtk-3.0/gtk.css":  "@define-color theme_selected_bg_color rgba(204, 34, 0, 0.5);\n",
}
