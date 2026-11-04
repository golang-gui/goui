package resource

var (
	CaptureHUD     = text("Show HUD", "显示 HUD", "HUDを表示", "HUD 표시", "HUD anzeigen", "Afficher le HUD", "Показать HUD", "إظهار HUD")
	CaptureHUDHint = text(
		"The HUD follows the main window across pages. It shows recent CPU frames, updates only when the app paints, and keeps its last reading while idle. It does not intercept clicks; its own drawing cost is included.",
		"HUD 随主窗口跨页显示，统计最近的 CPU 绘制帧，仅随应用绘制更新，静止时保留读数。浮层不拦截点击，自身绘制开销也计入统计。",
		"HUDはページ切替後も表示されます。最近のCPUフレームを描画時のみ更新し、待機中は値を保持します。クリックを妨げず、HUDの描画コストも含みます。",
		"HUD는 페이지를 바꿔도 표시됩니다. 앱이 그릴 때만 최근 CPU 프레임을 갱신하며 유휴 시 값을 유지합니다. 클릭을 가로채지 않으며 자체 그리기 비용도 포함합니다.",
		"Das HUD bleibt beim Seitenwechsel sichtbar. Es zeigt letzte CPU-Frames, aktualisiert sich nur beim Zeichnen und behält im Leerlauf die Werte. Klicks bleiben möglich; seine Zeichenkosten sind enthalten.",
		"Le HUD reste visible entre les pages. Il montre les dernières images CPU, se met à jour lors du dessin et conserve les valeurs au repos. Les clics passent ; son coût est inclus.",
		"HUD остаётся при смене страниц. Последние кадры CPU обновляются только при рисовании; в покое значения сохраняются. Клики проходят сквозь него; его затраты учтены.",
		"يبقى HUD ظاهراً بين الصفحات. يعرض إطارات المعالجة الأخيرة ويُحدّث عند الرسم فقط ويحفظ القراءة عند الخمول. لا يعترض النقرات وتُحتسب كلفة رسمه.",
	)
)
