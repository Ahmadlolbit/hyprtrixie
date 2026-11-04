package wallpaper

import (
	"bytes"
	"hash/fnv"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"runtime"
	"strconv"
	"sync"

	"github.com/Ahmadlolbit/hyprtrixie/internal/theme"
)

const (
	Width   = 2560
	Height  = 1600
	quality = 92
	step    = 0.0008
)

type rgb struct{ r, g, b float64 }

type noise struct{ seed uint32 }

func hex(value string) (rgb, bool) {
	if len(value) != 7 || value[0] != '#' {
		return rgb{}, false
	}
	number, err := strconv.ParseUint(value[1:], 16, 32)
	if err != nil {
		return rgb{}, false
	}
	return rgb{float64(number >> 16 & 0xff), float64(number >> 8 & 0xff), float64(number & 0xff)}, true
}

func pick(colors theme.Colors, names ...string) rgb {
	for _, name := range names {
		if value, ok := hex(colors[name]); ok {
			return value
		}
	}
	return rgb{128, 128, 128}
}

func mix(a, b rgb, t float64) rgb {
	t = math.Max(0, math.Min(1, t))
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

func smooth(edge0, edge1, x float64) float64 {
	t := math.Max(0, math.Min(1, (x-edge0)/(edge1-edge0)))
	return t * t * (3 - 2*t)
}

func (n noise) lattice(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263 + n.seed*2147483647
	h = (h ^ (h >> 13)) * 1274126177
	h ^= h >> 16
	return float64(h) / float64(math.MaxUint32)
}

func (n noise) value(x, y float64) float64 {
	fx, fy := math.Floor(x), math.Floor(y)
	ix, iy := int(fx), int(fy)
	tx, ty := smooth(0, 1, x-fx), smooth(0, 1, y-fy)
	top := n.lattice(ix, iy) + (n.lattice(ix+1, iy)-n.lattice(ix, iy))*tx
	bottom := n.lattice(ix, iy+1) + (n.lattice(ix+1, iy+1)-n.lattice(ix, iy+1))*tx
	return top + (bottom-top)*ty
}

func (n noise) fbm(x, y float64, octaves int) float64 {
	total, amplitude, norm := 0.0, 0.5, 0.0
	for range octaves {
		total += n.value(x, y) * amplitude
		norm += amplitude
		x, y = x*2.03+17.1, y*2.03+9.2
		amplitude *= 0.5
	}
	return total / norm
}

func Seed(text string) uint32 {
	sum := fnv.New32a()
	sum.Write([]byte(text))
	return sum.Sum32()
}

func depths(colors theme.Colors) (rgb, rgb) {
	base := pick(colors, "background")
	if colors["mode"] == "light" {
		return base, mix(base, pick(colors, "foreground"), 0.30)
	}
	return base, pick(colors, "darker_background", "dark_background", "background")
}

func contours(colors theme.Colors, n noise, aspect float64) func(x, y float64) rgb {
	base, deep := depths(colors)
	accent, second := pick(colors, "accent", "blue"), pick(colors, "cyan", "green", "accent")
	tilt := (n.value(3.7, 1.9) - 0.5) * 1.6
	scale := 1.7 + n.value(5.3, 7.1)*0.9
	return func(x, y float64) rgb {
		wash := mix(base, deep, smooth(0, 1, (x+y)/2))
		field := func(px, py float64) float64 {
			return n.fbm(px*aspect*scale, py*scale, 4)*9 + px*0.8 + py*tilt
		}
		here := field(x, y)
		slope := math.Hypot(field(x+step, y)-here, field(x, y+step)-here) / step
		nearest := math.Round(here)
		distance := math.Abs(here-nearest) / math.Max(slope, 0.5)
		thickness, strength := 0.00042, 0.30
		if math.Mod(nearest, 5) == 0 {
			thickness, strength = 0.00080, 0.62
		}
		line := 1 - smooth(thickness*0.55, thickness, distance)
		return mix(wash, mix(accent, second, n.value(x*2.5, y*2.5)), line*strength)
	}
}

func clamp(value float64) uint8 {
	return uint8(math.Max(0, math.Min(255, math.Round(value))))
}

func paintRow(img *image.RGBA, shade func(x, y float64) rgb, grain noise, y int) {
	width, height := img.Rect.Dx(), img.Rect.Dy()
	for x := range width {
		c := shade(float64(x)/float64(width), float64(y)/float64(height))
		d := (grain.lattice(x, y) - 0.5) * 2.4
		img.SetRGBA(x, y, color.RGBA{clamp(c.r + d), clamp(c.g + d), clamp(c.b + d), 255})
	}
}

func render(shade func(x, y float64) rgb, grain noise, width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	rows := make(chan int, height)
	for y := range height {
		rows <- y
	}
	close(rows)
	var group sync.WaitGroup
	for range runtime.NumCPU() {
		group.Add(1)
		go func() {
			defer group.Done()
			for y := range rows {
				paintRow(img, shade, grain, y)
			}
		}()
	}
	group.Wait()
	return img
}

func Contours(colors theme.Colors, seed uint32, width, height int) *image.RGBA {
	n := noise{seed}
	return render(contours(colors, n, float64(width)/float64(height)), noise{seed ^ 0x9e3779b9}, width, height)
}

func JPEG(img image.Image) ([]byte, error) {
	var out bytes.Buffer
	err := jpeg.Encode(&out, img, &jpeg.Options{Quality: quality})
	return out.Bytes(), err
}
