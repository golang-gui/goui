// Package resource contains framework-owned presentation text, not application
// content, stable semantic identifiers, diagnostics or native system dialogs.
package resource

import "github.com/golang-gui/goui/mlang"

var (
	Undo = mlang.String(mlang.En("Undo"),
		mlang.ZhCN("撤销"), mlang.Ja("元に戻す"), mlang.Ko("실행 취소"),
		mlang.De("Rückgängig"), mlang.Fr("Annuler"),
		mlang.Ru("Отменить"), mlang.Ar("تراجع"))
	Redo = mlang.String(mlang.En("Redo"),
		mlang.ZhCN("重做"), mlang.Ja("やり直す"), mlang.Ko("다시 실행"),
		mlang.De("Wiederholen"), mlang.Fr("Rétablir"),
		mlang.Ru("Повторить"), mlang.Ar("إعادة"))
	Cut = mlang.String(mlang.En("Cut"),
		mlang.ZhCN("剪切"), mlang.Ja("切り取り"), mlang.Ko("잘라내기"),
		mlang.De("Ausschneiden"), mlang.Fr("Couper"),
		mlang.Ru("Вырезать"), mlang.Ar("قص"))
	Copy = mlang.String(mlang.En("Copy"),
		mlang.ZhCN("复制"), mlang.Ja("コピー"), mlang.Ko("복사"),
		mlang.De("Kopieren"), mlang.Fr("Copier"),
		mlang.Ru("Копировать"), mlang.Ar("نسخ"))
	Paste = mlang.String(mlang.En("Paste"),
		mlang.ZhCN("粘贴"), mlang.Ja("貼り付け"), mlang.Ko("붙여넣기"),
		mlang.De("Einfügen"), mlang.Fr("Coller"),
		mlang.Ru("Вставить"), mlang.Ar("لصق"))
	Delete = mlang.String(mlang.En("Delete"),
		mlang.ZhCN("删除"), mlang.Ja("削除"), mlang.Ko("삭제"),
		mlang.De("Löschen"), mlang.Fr("Supprimer"),
		mlang.Ru("Удалить"), mlang.Ar("حذف"))
	SelectAll = mlang.String(mlang.En("Select All"),
		mlang.ZhCN("全选"), mlang.Ja("すべて選択"), mlang.Ko("모두 선택"),
		mlang.De("Alles auswählen"), mlang.Fr("Tout sélectionner"),
		mlang.Ru("Выделить всё"), mlang.Ar("تحديد الكل"))
	Minimize = mlang.String(mlang.En("Minimize"),
		mlang.ZhCN("最小化"), mlang.Ja("最小化"), mlang.Ko("최소화"),
		mlang.De("Minimieren"), mlang.Fr("Réduire"),
		mlang.Ru("Свернуть"), mlang.Ar("تصغير"))
	Maximize = mlang.String(mlang.En("Maximize"),
		mlang.ZhCN("最大化"), mlang.Ja("最大化"), mlang.Ko("최대화"),
		mlang.De("Maximieren"), mlang.Fr("Agrandir"),
		mlang.Ru("Развернуть"), mlang.Ar("تكبير"))
	Close = mlang.String(mlang.En("Close"),
		mlang.ZhCN("关闭"), mlang.Ja("閉じる"), mlang.Ko("닫기"),
		mlang.De("Schließen"), mlang.Fr("Fermer"),
		mlang.Ru("Закрыть"), mlang.Ar("إغلاق"))
	Restore = mlang.String(mlang.En("Restore"),
		mlang.ZhCN("还原"), mlang.Ja("元のサイズに戻す"), mlang.Ko("이전 크기로 복원"),
		mlang.De("Wiederherstellen"), mlang.Fr("Restaurer"),
		mlang.Ru("Восстановить"), mlang.Ar("استعادة"))
	CloseTab = mlang.String(mlang.En("Close tab"),
		mlang.ZhCN("关闭标签页"), mlang.Ja("タブを閉じる"), mlang.Ko("탭 닫기"),
		mlang.De("Tab schließen"), mlang.Fr("Fermer l’onglet"),
		mlang.Ru("Закрыть вкладку"), mlang.Ar("إغلاق علامة التبويب"))
	ScrollTabsLeft = mlang.String(mlang.En("Scroll tabs left"),
		mlang.ZhCN("向左滚动标签页"), mlang.Ja("タブを左にスクロール"), mlang.Ko("탭을 왼쪽으로 스크롤"),
		mlang.De("Tabs nach links scrollen"), mlang.Fr("Faire défiler les onglets vers la gauche"),
		mlang.Ru("Прокрутить вкладки влево"), mlang.Ar("تمرير علامات التبويب إلى اليسار"))
	ScrollTabsRight = mlang.String(mlang.En("Scroll tabs right"),
		mlang.ZhCN("向右滚动标签页"), mlang.Ja("タブを右にスクロール"), mlang.Ko("탭을 오른쪽으로 스크롤"),
		mlang.De("Tabs nach rechts scrollen"), mlang.Fr("Faire défiler les onglets vers la droite"),
		mlang.Ru("Прокрутить вкладки вправо"), mlang.Ar("تمرير علامات التبويب إلى اليمين"))
)
