package main

import (
	"os"
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/mleonidas/psqlconnect/pgpass"
	"github.com/mleonidas/psqlconnect/ui"
	"gopkg.in/natefinch/lumberjack.v2"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	connections         []*pgpass.Connection
	filteredConnections []*pgpass.Connection
	selectedIndex       = 0
	connectOnExit       = false
	filter              = ""
	logger              *zap.Logger
)

func createLogger() *zap.Logger {
	stdout := zapcore.AddSync(os.Stdout)

	file := zapcore.AddSync(&lumberjack.Logger{
		Filename:   "/tmp/psqlconnect.log",
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     7, // days
	})

	level := zap.NewAtomicLevelAt(zap.InfoLevel)

	productionCfg := zap.NewProductionEncoderConfig()
	productionCfg.TimeKey = "timestamp"
	productionCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	developmentCfg := zap.NewDevelopmentEncoderConfig()
	developmentCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder

	consoleEncoder := zapcore.NewConsoleEncoder(developmentCfg)
	fileEncoder := zapcore.NewJSONEncoder(productionCfg)

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, stdout, level),
		zapcore.NewCore(fileEncoder, file, level),
	)

	return zap.New(core)
}

func main() {
	logger = createLogger()
	defer logger.Sync()
	var err error
	connections, err = pgpass.LoadConnectionsFromPgpass()
	if err != nil {
		logger.Fatal("failed to load the connection pgpass", zap.Error(err))
	}
	filteredConnections = connections
	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		logger.Fatal("failed to setup gui", zap.Error(err))
	}
	g.Cursor = true
	g.SetManagerFunc(layout)
	err = keybindings(g)
	if err != nil {
		g.Close()
		logger.Fatal("failed to setup keybindings", zap.Error(err))
	}
	err = g.MainLoop()
	if err != nil && err != gocui.ErrQuit {
		g.Close()
		logger.Fatal("failed to run the mainloop", zap.Error(err))
	}
	g.Close()
	if connectOnExit {
		pgpass.ConnectToDatabase(filteredConnections[selectedIndex])
	}
}

func layout(g *gocui.Gui) error {
	err := ui.RenderHeaderView(g, len(filteredConnections), filter)
	err = ui.RenderConnectionsView(g, filteredConnections)
	err = ui.RenderInstructions(g)

	if err != nil {
		return err
	}

	return nil
}

func keybindings(g *gocui.Gui) error {
	err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return gocui.ErrQuit
	})

	err = g.SetKeybinding("connections", gocui.KeyArrowUp, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if selectedIndex != 0 {
			selectedIndex--
			v.MoveCursor(0, -1, false)
		}
		return nil
	})

	err = g.SetKeybinding("connections", 'k', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if selectedIndex != 0 {
			selectedIndex--
			v.MoveCursor(0, -1, false)
		}
		return nil
	})

	err = g.SetKeybinding("connections", gocui.KeyArrowDown, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if selectedIndex != len(filteredConnections)-1 {
			selectedIndex++
			v.MoveCursor(0, 1, false)
		}
		return nil
	})

	err = g.SetKeybinding("connections", 'j', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if selectedIndex != len(filteredConnections)-1 {
			selectedIndex++
			v.MoveCursor(0, 1, false)
		}
		return nil
	})

	err = g.SetKeybinding("connections", 'f', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		err := ui.RenderFilterView(g, filter)
		return err
	})

	err = g.SetKeybinding("connections", gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if len(filteredConnections) > 0 {
			connectOnExit = true
			return gocui.ErrQuit
		}
		return nil
	})

	err = g.SetKeybinding("connections", 'r', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		connections, err = pgpass.LoadConnectionsFromPgpass()

		if err != nil {
			return err
		}

		if len(filter) > 0 {
			filteredConnections = pgpass.GetFilteredConnections(connections, filter)
		} else {
			filteredConnections = connections
		}

		selectedIndex = 0
		err = v.SetCursor(0, 0)

		return err
	})

	err = g.SetKeybinding("filter", gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		buffer := v.Buffer()
		err := g.DeleteView("filter")
		if err != nil {
			return err
		}
		filter = strings.TrimSuffix(strings.TrimSpace(buffer), "\n")
		g.Highlight = false
		g.SelFgColor = gocui.ColorWhite
		cv, err := g.SetCurrentView("connections")
		if len(filter) > 0 {
			filteredConnections = pgpass.GetFilteredConnections(connections, filter)
		} else {
			filteredConnections = connections
		}
		selectedIndex = 0
		err = cv.SetCursor(0, 0)
		return err
	})

	return err
}
