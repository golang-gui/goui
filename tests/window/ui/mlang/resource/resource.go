// Package resource is this example's text and prepared icon bundle.
package resource

import (
	"strings"

	"github.com/golang-gui/goui/icon/svg"
	"github.com/golang-gui/goui/mlang"
)

var (
	Title = mlang.String(mlang.En("GOUI language resources"),
		mlang.ZhCN("GOUI 多语言资源"), mlang.Ja("GOUI 多言語リソース"), mlang.Ko("GOUI 다국어 리소스"),
		mlang.De("GOUI Sprachressourcen"), mlang.Fr("GOUI Ressources linguistiques"),
		mlang.Ru("GOUI Языковые ресурсы"), mlang.Ar("GOUI موارد اللغة"))
	Greeting = mlang.String(mlang.En("Hello — welcome!"),
		mlang.ZhCN("你好，欢迎使用！"), mlang.Ja("こんにちは、ようこそ！"), mlang.Ko("안녕하세요, 환영합니다!"),
		mlang.De("Hallo — willkommen!"), mlang.Fr("Bonjour — bienvenue !"),
		mlang.Ru("Здравствуйте — добро пожаловать!"), mlang.Ar("مرحباً — أهلاً وسهلاً!"))
	Save = mlang.String(mlang.En("Save"),
		mlang.ZhCN("保存"), mlang.Ja("保存"), mlang.Ko("저장"),
		mlang.De("Speichern"), mlang.Fr("Enregistrer"),
		mlang.Ru("Сохранить"), mlang.Ar("حفظ"))
	SaveIcon *svg.Source
)

// Load prepares content before ui.Run, not inside Build. Each Icon owns its
// native painter image; this bundle holds only shared CPU source data.
func Load() (err error) {
	SaveIcon, err = svg.Parse(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M4 2h13l5 5v15H2V2h2zm2 2v6h11V4H6zm0 10v6h12v-6H6z"/></svg>`))
	return
}
