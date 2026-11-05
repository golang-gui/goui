package resource

var (
	CommandEnabled = text("Enable shared command", "启用共享操作", "共有コマンドを有効化", "공유 명령 사용", "Gemeinsamen Befehl aktivieren", "Activer la commande partagée", "Включить общую команду", "تمكين الأمر المشترك")
	CommandCount   = text("Shared command executed: %d", "共享操作已执行：%d 次", "共有コマンドの実行回数：%d", "공유 명령 실행: %d회", "Gemeinsamer Befehl ausgeführt: %d", "Commande partagée exécutée : %d", "Общая команда выполнена: %d", "مرات تنفيذ الأمر المشترك: %d")
	CommandHint    = text(
		"The menu and F6 use one window command. F7 toggles its availability, even while the menu is open. Disabled commands ignore both entries; changing availability only updates bound properties.",
		"菜单与 F6 共用一个窗口操作。F7 切换启用状态，菜单打开时也会实时更新。禁用后两个入口均不执行；切换状态仅更新绑定属性。",
		"メニューと F6 は同じウィンドウコマンドを使います。F7 で有効状態を切り替えると、開いているメニューも更新されます。無効時はどちらも実行されず、状態変更はバインドした属性だけを更新します。",
		"메뉴와 F6는 같은 창 명령을 사용합니다. F7로 사용 여부를 바꾸면 열린 메뉴도 갱신됩니다. 비활성화하면 두 경로 모두 실행되지 않으며 바인딩된 속성만 갱신됩니다.",
		"Menü und F6 nutzen denselben Fensterbefehl. F7 schaltet ihn auch bei geöffnetem Menü um. Deaktiviert reagiert kein Zugang; nur gebundene Eigenschaften werden aktualisiert.",
		"Le menu et F6 utilisent la même commande de fenêtre. F7 change sa disponibilité, même dans le menu ouvert. Désactivée, aucune entrée ne l’exécute ; seules les propriétés liées sont mises à jour.",
		"Меню и F6 используют одну команду окна. F7 меняет её доступность даже в открытом меню. Отключённая команда не выполняется ни одним способом; обновляются только связанные свойства.",
		"تستخدم القائمة وF6 أمر نافذة واحداً. يغيّر F7 إتاحته حتى أثناء فتح القائمة. لا ينفّذ أي مدخل الأمر عند تعطيله؛ وتتغير الخصائص المرتبطة فقط.",
	)
	CornerTitle = text("Corner radii & box shadows", "四角圆角与盒形阴影", "四隅の角丸と影", "모서리 반경과 상자 그림자", "Eckradien und Schatten", "Rayons des coins et ombres", "Радиусы углов и тени", "أنصاف أقطار الزوايا وظلال الصندوق")
	CornerHint  = text(
		"Each shape uses four independent corner radii, one border width and color, and a matching box shadow. Drawing reserves space for the shadow; the style does not change window decoration.",
		"每个图形使用四个独立圆角、统一的边框宽度和颜色，以及匹配轮廓的盒形阴影。绘制主动预留阴影空间；样式不改变窗口装饰。",
		"各図形は独立した四隅の半径、共通の枠幅と色、輪郭に合う影を使います。描画側が影の余白を確保し、ウィンドウ装飾は変更しません。",
		"각 도형은 독립적인 네 모서리 반경, 공통 테두리 폭과 색, 윤곽에 맞는 그림자를 사용합니다. 그리기 영역에 그림자 공간을 확보하며 창 장식은 바꾸지 않습니다.",
		"Vier unabhängige Eckradien, eine gemeinsame Randbreite und Farbe sowie ein passender Schatten. Die Zeichnung reserviert Schattenraum; die Fensterdekoration bleibt unabhängig.",
		"Quatre rayons indépendants, une largeur et une couleur de bordure communes, et une ombre adaptée. Le dessin réserve la place de l’ombre sans modifier le décor de fenêtre.",
		"Четыре независимых радиуса, единые ширина и цвет рамки и тень по контуру. Место для тени выделяет рисунок; оформление окна не меняется.",
		"أربعة أنصاف أقطار مستقلة مع عرض ولون موحدين للحد وظل يطابق الشكل. يحجز الرسم مساحة للظل دون تغيير زخرفة النافذة.",
	)
	TraceHint = text(
		"Go execution tracing is independent of Capture and the HUD. Start with -trace goui.trace, close the demo normally, then run go tool trace goui.trace to inspect goui.frame tasks and layout/paint regions. The file is overwritten; it records runtime activity, not GPU presentation.",
		"Go 执行追踪独立于采集和 HUD。使用 -trace goui.trace 启动，正常关闭后运行 go tool trace goui.trace，可查看 goui.frame 任务及布局、绘制阶段。指定文件会被覆盖；记录运行时活动，不代表 GPU 呈现。",
		"Go 実行トレースは計測と HUD から独立しています。-trace goui.trace で起動し、正常終了後に go tool trace goui.trace を実行すると goui.frame と配置・描画区間を確認できます。ファイルは上書きされ、GPU 表示時刻ではなくランタイムの動作を記録します。",
		"Go 실행 추적은 수집 및 HUD와 독립적입니다. -trace goui.trace로 시작하고 정상 종료한 뒤 go tool trace goui.trace로 goui.frame과 배치·그리기 구간을 확인하세요. 파일은 덮어쓰며 GPU 표시가 아닌 런타임 활동을 기록합니다.",
		"Go-Tracing ist unabhängig von Aufnahme und HUD. Mit -trace goui.trace starten, normal schließen und go tool trace goui.trace ausführen: goui.frame sowie Layout- und Zeichenbereiche werden sichtbar. Die Datei wird überschrieben; sie enthält Laufzeitaktivität, keine GPU-Präsentation.",
		"La trace Go est indépendante de la capture et du HUD. Démarrez avec -trace goui.trace, fermez normalement puis lancez go tool trace goui.trace pour voir goui.frame et les phases de disposition et de dessin. Le fichier est écrasé ; il décrit l’exécution, pas la présentation GPU.",
		"Трассировка Go не зависит от сбора и HUD. Запустите с -trace goui.trace, закройте штатно и выполните go tool trace goui.trace: видны goui.frame и этапы компоновки и рисования. Файл перезаписывается; это активность среды выполнения, не показ кадров GPU.",
		"تتبّع Go مستقل عن التجميع وHUD. شغّل باستخدام -trace goui.trace، أغلق العرض بشكل طبيعي ثم نفّذ go tool trace goui.trace لفحص مهام goui.frame ومراحل التخطيط والرسم. يُستبدل الملف ويسجّل نشاط وقت التشغيل لا عرض GPU.",
	)
)
