package resource

var (
	TableLookFormat      = text("Table: %s", "表格：%s", "表：%s", "표: %s", "Tabelle: %s", "Tableau : %s", "Таблица: %s", "الجدول: %s")
	TableDefault         = text("Default", "默认外观", "標準", "기본 모양", "Standard", "Par défaut", "По умолчанию", "المظهر الافتراضي")
	TableCellGrid        = text("Cell grid", "单元格网格", "セルの格子", "셀 격자", "Zellraster", "Grille des cellules", "Сетка ячеек", "شبكة الخلايا")
	TableCellOutline     = text("Cell outline", "单元格轮廓", "セルの外枠", "셀 외곽선", "Zellumriss", "Contour des cellules", "Контур ячеек", "محيط الخلايا")
	TableScrollOutline   = text("Outline with scrollbars", "轮廓包含滚动条", "スクロールバーを含む外枠", "스크롤바 포함 외곽선", "Umriss mit Scrollleisten", "Contour avec barres de défilement", "Контур с полосами прокрутки", "محيط يشمل أشرطة التمرير")
	TableScrollGrid      = text("Grid with scrollbars", "网格包含滚动条", "スクロールバーを含む格子", "스크롤바 포함 격자", "Raster mit Scrollleisten", "Grille avec barres de défilement", "Сетка с полосами прокрутки", "شبكة تشمل أشرطة التمرير")
	DataPresentationHint = text(
		"Use Shift to select adjacent tree rows. Narrow the table to shrink columns, or choose how its borders surround cells and scrollbars.",
		"按住 Shift 选择连续树行。缩窄表格观察列收缩，也可切换边框包围单元格与滚动条的方式。",
		"Shift で連続したツリー行を選択。表を狭めて列の縮小を確認し、セルとスクロールバーを囲む枠を切り替えます。",
		"Shift로 연속된 트리 행을 선택하세요. 표를 좁혀 열 축소를 확인하고 셀과 스크롤바를 둘러싸는 테두리를 바꿔 보세요.",
		"Mit Shift benachbarte Baumzeilen auswählen. Die Tabelle verkleinern, um Spalten zu stauchen, oder den Rahmen um Zellen und Scrollleisten wechseln.",
		"Utilisez Maj pour sélectionner des lignes voisines. Réduisez le tableau pour rétrécir les colonnes ou changez le contour des cellules et des barres de défilement.",
		"С Shift выберите соседние строки дерева. Сузьте таблицу для сжатия столбцов или измените рамку вокруг ячеек и полос прокрутки.",
		"استخدم Shift لتحديد صفوف شجرية متجاورة. ضيّق الجدول لتقليص الأعمدة أو غيّر كيفية إحاطة الحدود بالخلايا وأشرطة التمرير.",
	)
	TextMultiClickHint = text(
		"Double-click selects a word; triple-click selects a logical line, or all text in a single-line input. Hold the third press and drag to select whole lines.",
		"双击选词；三击选中逻辑行，单行输入框则全选。第三次按下后拖动可按整行扩展选区。",
		"ダブルクリックで単語、トリプルクリックで論理行を選択。単行入力では全選択。3 回目を押したままドラッグすると行単位で選択できます。",
		"두 번 클릭하면 단어, 세 번 클릭하면 논리적 줄을 선택합니다. 한 줄 입력은 전체 선택됩니다. 세 번째 누름을 유지하고 끌면 줄 단위로 선택합니다.",
		"Doppelklick wählt ein Wort, Dreifachklick eine logische Zeile oder die gesamte einzeilige Eingabe. Beim dritten Drücken ziehen, um ganze Zeilen auszuwählen.",
		"Double-cliquez pour un mot, triple-cliquez pour une ligne logique ou toute la saisie simple. Maintenez le troisième appui et glissez pour sélectionner des lignes entières.",
		"Двойной щелчок выбирает слово, тройной — логическую строку или всё однострочное поле. После третьего нажатия тяните для выделения целых строк.",
		"النقر المزدوج يحدد كلمة والثلاثي يحدد سطراً منطقياً أو كامل حقل السطر الواحد. أبقِ الضغطة الثالثة واسحب لتحديد أسطر كاملة.",
	)
)
