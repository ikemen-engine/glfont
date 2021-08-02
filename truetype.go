package glfont

import (
	"fmt"
	"image"
	"image/draw"
	"io"
	"io/ioutil"

	"github.com/go-gl/gl/all-core/gl"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

type character struct {
	x, y     int
	width    int //glyph width
	height   int //glyph height
	advance  int //glyph advance
	bearingH int //glyph bearing horizontal
	bearingV int //glyph bearing vertical
}

func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

//LoadTrueTypeFont builds a set of textures based on a ttf files gylphs
func LoadTrueTypeFont(program uint32, r io.Reader, scale int32, low, high rune, dir Direction) (*Font, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Read the truetype font.
	ttf, err := truetype.Parse(data)
	if err != nil {
		return nil, err
	}

	//make Font stuct type
	f := new(Font)
	f.fontChar = make([]*character, 0, high-low+1)
	f.program = program            //set shader program
	f.SetColor(1.0, 1.0, 1.0, 1.0) //set default white

	//create new face
	ttfFace := truetype.NewFace(ttf, &truetype.Options{
		Size:    float64(scale),
		DPI:     72,
		Hinting: font.HintingFull,
	})

	var lineHeight float32
	f.atlasWidth = 1024
	f.atlasHeight = 1024
	for ch := low; ch <= high; ch++ {
		gBnd, _, ok := ttfFace.GlyphBounds(ch)
		if ok != true {
			return nil, fmt.Errorf("ttf face glyphBounds error")
		}
		gh := int32((gBnd.Max.Y - gBnd.Min.Y) >> 6)
		lineHeight = max(lineHeight, float32(gh))
	}

	//create image to draw glyph
	fg, bg := image.White, image.Black
	rect := image.Rect(0, 0, int(f.atlasWidth), int(f.atlasHeight))
	rgba := image.NewRGBA(rect)
	draw.Draw(rgba, rgba.Bounds(), bg, image.ZP, draw.Src)

	margin := 2
	x := margin
	y := margin

	//make each gylph
	for ch := low; ch <= high; ch++ {
		char := new(character)

		gBnd, gAdv, ok := ttfFace.GlyphBounds(ch)
		if ok != true {
			return nil, fmt.Errorf("ttf face glyphBounds error")
		}

		gh := int32((gBnd.Max.Y - gBnd.Min.Y) >> 6)
		gw := int32((gBnd.Max.X - gBnd.Min.X) >> 6)

		//if gylph has no dimensions set to a max value
		if gw == 0 || gh == 0 {
			gBnd = ttf.Bounds(fixed.Int26_6(scale))
			gw = int32((gBnd.Max.X - gBnd.Min.X) >> 6)
			gh = int32((gBnd.Max.Y - gBnd.Min.Y) >> 6)

			//above can sometimes yield 0 for font smaller than 48pt, 1 is minimum
			if gw == 0 || gh == 0 {
				gw = 1
				gh = 1
			}
		}

		//The glyph's ascent and descent equal -bounds.Min.Y and +bounds.Max.Y.
		gAscent := int(-gBnd.Min.Y) >> 6
		gdescent := int(gBnd.Max.Y) >> 6

		//set w,h and adv, bearing V and bearing H in char
		char.x = x
		char.y = y
		char.width = int(gw)
		char.height = int(gh)
		char.advance = int(gAdv)
		char.bearingV = gdescent
		char.bearingH = (int(gBnd.Min.X) >> 6)

		clip := image.Rect(x, y, x+int(gw), y+int(gh))

		//create a freetype context for drawing
		c := freetype.NewContext()
		c.SetDPI(72)
		c.SetFont(ttf)
		c.SetFontSize(float64(scale))
		c.SetClip(clip)
		c.SetDst(rgba)
		c.SetSrc(fg)
		c.SetHinting(font.HintingFull)

		//set the glyph dot
		px := 0 - (int(gBnd.Min.X) >> 6) + x
		py := (gAscent) + y
		pt := freetype.Pt(px, py)

		x += int(gw) + margin
		if x+int(gw)+margin > int(f.atlasWidth) {
			x = 0
			y += int(lineHeight) + margin
		}

		// Draw the text from mask to image
		_, err = c.DrawString(string(ch), pt)
		if err != nil {
			return nil, err
		}

		//add char to fontChar list
		f.fontChar = append(f.fontChar, char)
	}

	// Generate texture
	gl.GenTextures(1, &f.textureID)
	gl.BindTexture(gl.TEXTURE_2D, f.textureID)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(rgba.Rect.Dx()), int32(rgba.Rect.Dy()), 0,
		gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))

	gl.GenerateMipmap(gl.TEXTURE_2D)
	gl.BindTexture(gl.TEXTURE_2D, 0)

	// Configure VAO/VBO for texture quads
	gl.GenVertexArrays(1, &f.vao)
	gl.GenBuffers(1, &f.vbo)
	gl.BindVertexArray(f.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, f.vbo)

	vertAttrib := uint32(gl.GetAttribLocation(f.program, gl.Str("vert\x00")))
	gl.EnableVertexAttribArray(vertAttrib)
	gl.VertexAttribPointer(vertAttrib, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(0))

	texCoordAttrib := uint32(gl.GetAttribLocation(f.program, gl.Str("vertTexCoord\x00")))
	gl.EnableVertexAttribArray(texCoordAttrib)
	gl.VertexAttribPointer(texCoordAttrib, 2, gl.FLOAT, false, 4*4, gl.PtrOffset(2*4))

	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindVertexArray(0)

	return f, nil
}
