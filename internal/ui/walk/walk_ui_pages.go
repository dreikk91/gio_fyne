//go:build windows

package walk

import (
	"strconv"

	. "github.com/lxn/walk/declarative"
)

func (a *walkApp) objectsPage() TabPage {
	return TabPage{
		Title: "Об'єкти",
		Background: SolidColorBrush{
			Color: colorWindow,
		},
		Layout: VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 8},
		Children: []Widget{
			Composite{
				Background: SolidColorBrush{Color: colorSurface},
				AssignTo:   &a.objToolbar,
				Layout:     HBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}, Spacing: 8},
				Children: []Widget{
					LineEdit{
						AssignTo:      &a.objSearch,
						StretchFactor: 1,
						CueBanner:     "Пошук об'єкта",
						ToolTipText:   "Пошук по номеру ППК, адресі клієнта або тексту останньої події.",
						OnTextChanged: func() {
							a.deviceFilter = a.objSearch.Text()
							a.refreshUI()
						},
					},
					PushButton{Text: "Відкрити історію", OnClicked: a.openSelectedHistory},
					PushButton{Text: "Видалити об'єкт", OnClicked: a.deleteSelectedDevice},
					PushButton{Text: "Оновити", OnClicked: a.reloadAll},
				},
			},
			TableView{
				Background:          SolidColorBrush{Color: colorSurface},
				AssignTo:            &a.objTable,
				AlternatingRowBG:    true,
				ColumnsOrderable:    true,
				LastColumnStretched: false,
				CustomHeaderHeight:  34,
				CustomRowHeight:     32,
				Model:               a.deviceModel,
				StyleCell:           a.styleDeviceCell,
				OnItemActivated:     a.openSelectedHistory,
				OnSizeChanged:       a.updateObjectTableColumns,
				Columns: []TableViewColumn{
					{Title: "Стан", Width: 82},
					{Title: "ППК", Width: 72},
					{Title: "Клієнт", Width: 140},
					{Title: "Остання подія", Width: 280},
					{Title: "Дата/Час", Width: 154},
				},
			},
		},
	}
}

func (a *walkApp) eventsPage() TabPage {
	return TabPage{
		Title: "Події",
		Background: SolidColorBrush{
			Color: colorWindow,
		},
		Layout: VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 8},
		Children: []Widget{
			Composite{
				Background: SolidColorBrush{Color: colorSurface},
				AssignTo:   &a.eventToolbar,
				Layout:     HBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}, Spacing: 8},
				Children: []Widget{
					ComboBox{
						AssignTo:    &a.eventFilterBox,
						Model:       eventFilters,
						ToolTipText: "Фільтр категорії для глобального журналу подій.",
						OnCurrentIndexChanged: func() {
							if a.eventFilterBox.CurrentIndex() >= 0 {
								a.eventFilter = eventFilters[a.eventFilterBox.CurrentIndex()]
								a.refreshUI()
							}
						},
					},
					CheckBox{
						AssignTo:    &a.hideTestsBox,
						Text:        "Приховати тестові",
						ToolTipText: "Приховує події з категорією test у глобальному журналі.",
						OnCheckedChanged: func() {
							a.hideTests = a.hideTestsBox.Checked()
							a.refreshUI()
						},
					},
					LineEdit{
						AssignTo:      &a.eventSearch,
						StretchFactor: 1,
						CueBanner:     "Пошук подій",
						ToolTipText:   "Пошук по ППК, коду, типу, опису або зоні.",
						OnTextChanged: func() {
							a.eventQuery = a.eventSearch.Text()
							a.refreshUI()
						},
					},
					PushButton{Text: "Більше", OnClicked: a.loadMoreEvents},
					PushButton{Text: "Повне оновлення", OnClicked: a.reloadAll},
				},
			},
			TableView{
				Background:          SolidColorBrush{Color: colorSurface},
				AssignTo:            &a.eventTable,
				AlternatingRowBG:    true,
				ColumnsOrderable:    true,
				LastColumnStretched: false,
				CustomHeaderHeight:  34,
				CustomRowHeight:     32,
				Model:               a.eventModel,
				StyleCell:           a.styleEventCell,
				OnSizeChanged:       a.updateEventTableColumns,
				Columns: []TableViewColumn{
					{Title: "Час", Width: 140},
					{Title: "ППК", Width: 75},
					{Title: "Код", Width: 60},
					{Title: "Тип", Width: 120},
					{Title: "Опис", Width: 280},
					{Title: "Зона/Група", Width: 160, Alignment: AlignCenter},
					{Title: "Категорія", Width: 90, Alignment: AlignCenter},
				},
			},
		},
	}
}

func (a *walkApp) settingsPage() TabPage {
	return TabPage{
		Title: "Налаштування",
		Background: SolidColorBrush{
			Color: colorWindow,
		},
		Layout: VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}},
		Children: []Widget{
			ScrollView{
				Background: SolidColorBrush{Color: colorWindow},
				Layout:     VBox{Spacing: 10},
				Children: []Widget{
					GroupBox{
						Title:       "Мережа",
						ToolTipText: "Параметри TCP сервера, клієнта та черги повідомлень.",
						Background:  SolidColorBrush{Color: colorSurface},
						Layout:      Grid{Columns: 2, Spacing: 8},
						Children: []Widget{
							Label{Text: "Server host"},
							LineEdit{AssignTo: &a.serverHost, ToolTipText: "IP або hostname локального TCP сервера для прийому CID повідомлень."},
							Label{Text: "Server port"},
							LineEdit{AssignTo: &a.serverPort, ToolTipText: "Порт локального сервера для вхідних підключень."},
							Label{Text: "Client host"},
							LineEdit{AssignTo: &a.clientHost, ToolTipText: "Адреса віддаленого ретранслятора, куди пересилаються повідомлення."},
							Label{Text: "Client port"},
							LineEdit{AssignTo: &a.clientPort, ToolTipText: "Порт віддаленого ретранслятора."},
							Label{Text: "Reconnect initial"},
							LineEdit{AssignTo: &a.reconnectInit, ToolTipText: "Початкова затримка перепідключення, наприклад 1s або 5s."},
							Label{Text: "Reconnect max"},
							LineEdit{AssignTo: &a.reconnectMax, ToolTipText: "Максимальна затримка між спробами перепідключення."},
							Label{Text: "Queue buffer"},
							LineEdit{AssignTo: &a.queueBuffer, ToolTipText: "Розмір буфера черги повідомлень у пам'яті."},
							Label{Text: "PPK timeout"},
							LineEdit{AssignTo: &a.ppkTimeout, ToolTipText: "Через який час без подій об'єкт вважається неактивним."},
						},
					},
					GroupBox{
						Title:       "Логування",
						ToolTipText: "Налаштування файлів журналу та рівня деталізації логування.",
						Background:  SolidColorBrush{Color: colorSurface},
						Layout:      Grid{Columns: 2, Spacing: 8},
						Children: []Widget{
							Label{Text: "Log dir"},
							LineEdit{AssignTo: &a.logDir, ToolTipText: "Папка, де зберігатимуться файли логів."},
							Label{Text: "Filename"},
							LineEdit{AssignTo: &a.logFilename, ToolTipText: "Базове ім'я файла журналу."},
							Label{Text: "Max size"},
							LineEdit{AssignTo: &a.logMaxSize, ToolTipText: "Максимальний розмір файла логу в МБ перед ротацією."},
							Label{Text: "Max backups"},
							LineEdit{AssignTo: &a.logMaxBackups, ToolTipText: "Скільки архівних копій логів зберігати."},
							Label{Text: "Max age"},
							LineEdit{AssignTo: &a.logMaxAge, ToolTipText: "Скільки днів тримати старі логи."},
							Label{Text: "Level"},
							ComboBox{AssignTo: &a.logLevel, Model: logLevels, ToolTipText: "Рівень логування від trace до fatal."},
							CheckBox{AssignTo: &a.logConsole, Text: "Enable console", ToolTipText: "Вивід логів у консоль."},
							CheckBox{AssignTo: &a.logFile, Text: "Enable file", ToolTipText: "Запис логів у файл."},
							CheckBox{AssignTo: &a.logPretty, Text: "Pretty console", ToolTipText: "Більш читабельний формат логів у консолі."},
							CheckBox{AssignTo: &a.logSampling, Text: "Sampling enabled", ToolTipText: "Обмеження потоку однакових логів при високому навантаженні."},
						},
					},
					GroupBox{
						Title:       "Історія та UI",
						ToolTipText: "Ліміти журналу, архівація та поведінка інтерфейсу.",
						Background:  SolidColorBrush{Color: colorSurface},
						Layout:      Grid{Columns: 2, Spacing: 8},
						Children: []Widget{
							Label{Text: "History global"},
							LineEdit{AssignTo: &a.historyGlobal, ToolTipText: "Скільки записів максимум тримати в глобальному журналі подій."},
							Label{Text: "History log"},
							LineEdit{AssignTo: &a.historyLog, ToolTipText: "Початковий ліміт історії при відкритті журналу окремого об'єкта."},
							Label{Text: "Retention days"},
							LineEdit{AssignTo: &a.historyRetention, ToolTipText: "Скільки днів зберігати історію в основній БД."},
							Label{Text: "Cleanup interval (hours)"},
							LineEdit{AssignTo: &a.historyCleanup, ToolTipText: "Інтервал запуску обслуговування та очищення БД."},
							Label{Text: "Archive DB path"},
							LineEdit{AssignTo: &a.historyArchivePath, ToolTipText: "Файл archive DB для старих подій."},
							Label{Text: "Maintenance batch"},
							LineEdit{AssignTo: &a.historyBatch, ToolTipText: "Розмір пакета при архівації або очищенні історії."},
							CheckBox{AssignTo: &a.historyArchive, Text: "Archive enabled", ToolTipText: "Переміщати старі події в archive DB."},
							CheckBox{AssignTo: &a.uiStartMin, Text: "Start minimized", ToolTipText: "Запускати програму згорнутою."},
							CheckBox{AssignTo: &a.uiMinTray, Text: "Minimize to tray", ToolTipText: "Ховати вікно в трей при мінімізації."},
							CheckBox{AssignTo: &a.uiCloseTray, Text: "Close to tray", ToolTipText: "Ховати вікно в трей при закритті замість завершення."},
							Label{Text: "Font size"},
									ComboBox{
										AssignTo: &a.uiFontCombo,
										Model:    []string{"8", "9", "10", "11", "12", "14", "16", "18", "20", "22", "24", "26", "28", "30"},
										OnCurrentIndexChanged: func() {
											if a.uiFontValue != nil && a.uiFontCombo != nil {
												valStr := a.uiFontCombo.Text()
												val, _ := strconv.Atoi(valStr)
												if val > 0 {
													a.uiFontValue.SetText(valStr)
													a.applyUIFont(val)
												}
											}
										},
									},
									Label{AssignTo: &a.uiFontValue, Text: "14"},
						},
					},
					GroupBox{
						Title: "Додатково",
						Background: SolidColorBrush{Color: colorSurface},
						Layout: HBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}, Spacing: 8},
						Children: []Widget{
							PushButton{Text: "Налаштування фільтрації", OnClicked: a.openRelayFilter},
							PushButton{Text: "Кольори подій", OnClicked: a.openColorSettings},
							HSpacer{},
						},
					},
					GroupBox{
						Title:       "CID правила",
						ToolTipText: "Правила валідації та трансформації account number.",
						Background:  SolidColorBrush{Color: colorSurface},
						Layout:      Grid{Columns: 2, Spacing: 8},
						Children: []Widget{
							Label{Text: "Required prefix"},
							LineEdit{AssignTo: &a.requiredPrefix, ToolTipText: "Очікуваний префікс CID повідомлення."},
							Label{Text: "Valid length"},
							LineEdit{AssignTo: &a.validLength, ToolTipText: "Очікувана довжина валідного CID повідомлення."},
							Label{Text: "Account ranges (From-To:Delta)"},
							TextEdit{AssignTo: &a.accountRanges, MinSize: Size{Width: 300, Height: 120}, ToolTipText: "Кожен рядок у форматі 2000-2200:+2100. Саме ці правила застосовуються для зміни account number."},
						},
					},
					Composite{
						Layout: HBox{Spacing: 8},
						Children: []Widget{
							HSpacer{},
							PushButton{Text: "Скинути", OnClicked: a.loadConfigEditors},
							PushButton{Text: "Зберегти", OnClicked: a.saveConfig},
						},
					},
				},
			},
		},
	}
}
