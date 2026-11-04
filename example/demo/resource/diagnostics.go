package resource

var (
	CaptureHUD     = text("Show HUD", "显示 HUD", "HUDを表示", "HUD 표시", "HUD anzeigen", "Afficher le HUD", "Показать HUD", "إظهار HUD")
	CaptureHUDHint = text(
		"The HUD follows the main window across pages and shows recent CPU frames. After one idle second it repaints once to show 0 FPS, then stops refreshing. It does not intercept clicks; its own drawing cost is included.",
		"HUD 随主窗口跨页显示，统计最近的 CPU 绘制帧；静止一秒后仅重绘一次将 FPS 清零，随后停止刷新。浮层不拦截点击，自身绘制开销也计入统计。",
		"HUDはページ切替後も最近のCPUフレームを表示します。1秒間描画がなければ一度だけ更新して0 FPSを表示し、更新を停止します。クリックを妨げず、描画コストも含みます。",
		"HUD는 페이지를 바꿔도 최근 CPU 프레임을 표시합니다. 1초간 유휴 상태면 한 번만 그려 0 FPS로 바꾼 뒤 갱신을 멈춥니다. 클릭을 가로채지 않으며 자체 그리기 비용도 포함합니다.",
		"Das HUD zeigt letzte CPU-Frames auf allen Seiten. Nach einer Sekunde ohne Zeichnen zeigt eine einzelne Aktualisierung 0 FPS; danach stoppt sie. Klicks bleiben möglich; seine Zeichenkosten sind enthalten.",
		"Le HUD montre les dernières images CPU entre les pages. Après une seconde au repos, il se redessine une fois pour afficher 0 FPS, puis s’arrête. Les clics passent ; son coût est inclus.",
		"HUD показывает последние кадры CPU на всех страницах. После секунды покоя он рисуется один раз для показа 0 FPS, затем прекращает обновления. Клики проходят сквозь него; его затраты учтены.",
		"يعرض HUD إطارات المعالجة الأخيرة بين الصفحات. بعد ثانية من الخمول يُرسم مرة واحدة لعرض 0 FPS ثم يتوقف التحديث. لا يعترض النقرات وتُحتسب كلفة رسمه.",
	)
)
