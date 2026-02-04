package gfx

import (
	"errors"
	"image/color"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var errNotRunning = errors.New("gfx backend not running (use `welle gfx <file>`)")

type LoopFuncs struct {
	Setup  func() error
	Update func(dt float64) error
	Draw   func() error
}

type state struct {
	mu          sync.Mutex
	width       int
	height      int
	title       string
	commands    []command
	clear       color.RGBA
	presentTex  *ebiten.Image
	presentW    int
	presentH    int
	start       time.Time
	lastTime    time.Time
	shouldClose bool
}

type command interface {
	draw(dst *ebiten.Image)
}

type rectCmd struct {
	x, y, w, h float32
	c          color.RGBA
}

func (r rectCmd) draw(dst *ebiten.Image) {
	vector.DrawFilledRect(dst, r.x, r.y, r.w, r.h, r.c, false)
}

type pixelCmd struct {
	x, y int
	c    color.RGBA
}

func (p pixelCmd) draw(dst *ebiten.Image) {
	vector.DrawFilledRect(dst, float32(p.x), float32(p.y), 1, 1, p.c, false)
}

type presentCmd struct {
	tex *ebiten.Image
	w   int
	h   int
}

func (p presentCmd) draw(dst *ebiten.Image) {
	if p.tex == nil || p.w <= 0 || p.h <= 0 {
		return
	}
	sw, sh := dst.Size()
	if sw == 0 || sh == 0 {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(sw)/float64(p.w), float64(sh)/float64(p.h))
	dst.DrawImage(p.tex, op)
}

var (
	stateMu sync.Mutex
	cur     *state
)

func Run(loop LoopFuncs) error {
	s := &state{
		width:  640,
		height: 480,
		title:  "Welle",
		clear:  color.RGBA{A: 255},
	}
	stateMu.Lock()
	cur = s
	stateMu.Unlock()
	defer func() {
		stateMu.Lock()
		cur = nil
		stateMu.Unlock()
	}()

	if loop.Setup != nil {
		if err := loop.Setup(); err != nil {
			return err
		}
	}

	s.mu.Lock()
	s.start = time.Now()
	s.lastTime = s.start
	width := s.width
	height := s.height
	title := s.title
	s.mu.Unlock()

	ebiten.SetWindowSize(width, height)
	ebiten.SetWindowTitle(title)

	game := &ebitenGame{loop: loop, state: s}
	return ebiten.RunGame(game)
}

type ebitenGame struct {
	loop  LoopFuncs
	state *state
}

func (g *ebitenGame) Update() error {
	s := g.state
	s.mu.Lock()
	now := time.Now()
	dt := now.Sub(s.lastTime).Seconds()
	s.lastTime = now
	s.mu.Unlock()

	if g.loop.Update != nil {
		if err := g.loop.Update(dt); err != nil {
			return err
		}
	}
	if g.loop.Draw != nil {
		if err := g.loop.Draw(); err != nil {
			return err
		}
	}

	s.mu.Lock()
	s.shouldClose = s.shouldClose || ebiten.IsWindowBeingClosed()
	s.mu.Unlock()
	if ShouldClose() {
		return ebiten.Termination
	}
	return nil
}

func (g *ebitenGame) Draw(screen *ebiten.Image) {
	s := g.state
	s.mu.Lock()
	clear := s.clear
	cmds := append([]command(nil), s.commands...)
	s.mu.Unlock()

	screen.Fill(clear)
	for _, cmd := range cmds {
		cmd.draw(screen)
	}
}

func (g *ebitenGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	s := g.state
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.width, s.height
}

func Open(width, height int, title string) error {
	s, err := getState()
	if err != nil {
		return err
	}
	if width <= 0 || height <= 0 {
		return errors.New("gfx_open expects positive width/height")
	}
	s.mu.Lock()
	s.width = width
	s.height = height
	if title != "" {
		s.title = title
	}
	s.mu.Unlock()
	ebiten.SetWindowSize(width, height)
	if title != "" {
		ebiten.SetWindowTitle(title)
	}
	return nil
}

func Close() error {
	s, err := getState()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.shouldClose = true
	s.mu.Unlock()
	return nil
}

func ShouldClose() bool {
	s, err := getState()
	if err != nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shouldClose
}

func BeginFrame() error {
	s, err := getState()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.commands = s.commands[:0]
	s.clear = color.RGBA{A: 255}
	s.mu.Unlock()
	return nil
}

func EndFrame() error {
	_, err := getState()
	return err
}

func Clear(r, g, b, a float64) error {
	s, err := getState()
	if err != nil {
		return err
	}
	c, err := rgbaFromNumbers(r, g, b, a)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.clear = c
	s.mu.Unlock()
	return nil
}

func Rect(x, y, w, h float64, r, g, b, a float64) error {
	s, err := getState()
	if err != nil {
		return err
	}
	c, err := rgbaFromNumbers(r, g, b, a)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.commands = append(s.commands, rectCmd{
		x: float32(x),
		y: float32(y),
		w: float32(w),
		h: float32(h),
		c: c,
	})
	s.mu.Unlock()
	return nil
}

func Pixel(x, y int, r, g, b, a int) error {
	s, err := getState()
	if err != nil {
		return err
	}
	c, err := rgbaFromInts(r, g, b, a)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.commands = append(s.commands, pixelCmd{x: x, y: y, c: c})
	s.mu.Unlock()
	return nil
}

func PresentRGBA(width, height int, data []uint8) error {
	s, err := getState()
	if err != nil {
		return err
	}
	if width <= 0 || height <= 0 {
		return errors.New("gfx_present expects positive width/height")
	}
	if len(data) != width*height*4 {
		return errors.New("gfx_present expects RGBA data sized to width*height*4")
	}
	s.mu.Lock()
	if s.presentTex == nil || s.presentW != width || s.presentH != height {
		s.presentTex = ebiten.NewImage(width, height)
		s.presentW = width
		s.presentH = height
	}
	s.presentTex.ReplacePixels(data)
	s.commands = append(s.commands, presentCmd{tex: s.presentTex, w: width, h: height})
	s.mu.Unlock()
	return nil
}

func TimeSeconds() (float64, error) {
	s, err := getState()
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.start.IsZero() {
		return 0, nil
	}
	return time.Since(s.start).Seconds(), nil
}

func KeyDown(key string) (bool, error) {
	_, err := getState()
	if err != nil {
		return false, err
	}
	k, ok := keyMap[strings.ToLower(key)]
	if !ok {
		return false, errors.New("unknown key: " + key)
	}
	return ebiten.IsKeyPressed(k), nil
}

func MouseX() (int, error) {
	_, err := getState()
	if err != nil {
		return 0, err
	}
	x, _ := ebiten.CursorPosition()
	return x, nil
}

func MouseY() (int, error) {
	_, err := getState()
	if err != nil {
		return 0, err
	}
	_, y := ebiten.CursorPosition()
	return y, nil
}

func getState() (*state, error) {
	stateMu.Lock()
	defer stateMu.Unlock()
	if cur == nil {
		return nil, errNotRunning
	}
	return cur, nil
}

func rgbaFromNumbers(r, g, b, a float64) (color.RGBA, error) {
	ri, err := toByte(r)
	if err != nil {
		return color.RGBA{}, err
	}
	gi, err := toByte(g)
	if err != nil {
		return color.RGBA{}, err
	}
	bi, err := toByte(b)
	if err != nil {
		return color.RGBA{}, err
	}
	ai, err := toByte(a)
	if err != nil {
		return color.RGBA{}, err
	}
	return color.RGBA{R: ri, G: gi, B: bi, A: ai}, nil
}

func rgbaFromInts(r, g, b, a int) (color.RGBA, error) {
	if r < 0 || r > 255 || g < 0 || g > 255 || b < 0 || b > 255 || a < 0 || a > 255 {
		return color.RGBA{}, errors.New("color channels must be 0..255")
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, nil
}

func toByte(v float64) (uint8, error) {
	if v < 0 || v > 255 || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, errors.New("color channels must be 0..255")
	}
	return uint8(math.Round(v)), nil
}

var keyMap = map[string]ebiten.Key{
	"space":  ebiten.KeySpace,
	"enter":  ebiten.KeyEnter,
	"escape": ebiten.KeyEscape,
	"left":   ebiten.KeyArrowLeft,
	"right":  ebiten.KeyArrowRight,
	"up":     ebiten.KeyArrowUp,
	"down":   ebiten.KeyArrowDown,
	"shift":  ebiten.KeyShift,
	"ctrl":   ebiten.KeyControl,
	"alt":    ebiten.KeyAlt,
}

func init() {
	for ch := 'a'; ch <= 'z'; ch++ {
		keyMap[string(ch)] = ebiten.KeyA + ebiten.Key(ch-'a')
	}
	for ch := '0'; ch <= '9'; ch++ {
		keyMap[string(ch)] = ebiten.Key0 + ebiten.Key(ch-'0')
	}
}
