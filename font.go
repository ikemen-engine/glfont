package glfont

import (
	"fmt"
	"os"

	"github.com/go-gl/gl/v2.1/gl"
)

// Direction represents the direction in which strings should be rendered.
type Direction uint8

// Known directions.
const (
	LeftToRight Direction = iota // E.g.: Latin
	RightToLeft                  // E.g.: Arabic
	TopToBottom                  // E.g.: Chinese
)

// A Font allows rendering of text to an OpenGL context.
type Font struct {
	fontChar    []*character
	vao         uint32
	vbo         uint32
	program     uint32
	textureID   uint32 // Holds the glyph texture id.
	color       color
	atlasWidth  float32
	atlasHeight float32
}

type color struct {
	r float32
	g float32
	b float32
	a float32
}

type point [4]float32

//LoadFont loads the specified font at the given scale.
func LoadFont(file string, scale int32, windowWidth int, windowHeight int, GLSLVersion uint) (*Font, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	// Configure the default font vertex and fragment shaders
	program, err := newProgram(GLSLVersion, vertexFontShader, fragmentFontShader)
	if err != nil {
		panic(err)
	}

	// Activate corresponding render state
	gl.UseProgram(program)

	//set screen resolution
	resUniform := gl.GetUniformLocation(program, gl.Str("resolution\x00"))
	gl.Uniform2f(resUniform, float32(windowWidth), float32(windowHeight))

	return LoadTrueTypeFont(program, fd, scale, 32, 256, LeftToRight)
}

//SetColor allows you to set the text color to be used when you draw the text
func (f *Font) SetColor(red float32, green float32, blue float32, alpha float32) {
	f.color.r = red
	f.color.g = green
	f.color.b = blue
	f.color.a = alpha
}

// UpdateResolution passes the new framebuffer size to the font shader
func (f *Font) UpdateResolution(windowWidth int, windowHeight int) {
	gl.UseProgram(f.program)
	resUniform := gl.GetUniformLocation(f.program, gl.Str("resolution\x00"))
	gl.Uniform2f(resUniform, float32(windowWidth), float32(windowHeight))
	gl.UseProgram(0)
}

//Printf draws a string to the screen, takes a list of arguments like printf
func (f *Font) Printf(x, y float32, scale float32, align int32, blend bool, window [4]int32, fs string, argv ...interface{}) error {

	indices := []rune(fmt.Sprintf(fs, argv...))

	if len(indices) == 0 {
		return nil
	}

	lowChar := rune(32)

	//setup blending mode
	gl.Enable(gl.BLEND)
	if blend {
		gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	}

	//restrict drawing to a certain part of the window	
	gl.Enable(gl.SCISSOR_TEST)
	gl.Scissor(window[0], window[1], window[2], window[3])

	// Activate corresponding render state
	gl.UseProgram(f.program)
	//set text color
	gl.Uniform4f(gl.GetUniformLocation(f.program, gl.Str("textColor\x00")), f.color.r, f.color.g, f.color.b, f.color.a)

	var coords []point

	//calculate alignment position
	if align == 0 {
		x -= f.Width(scale, fs) * 0.5
	} else if align < 0 {
		x -= f.Width(scale, fs)
	}

	// Iterate through all characters in string
	for i := range indices {
		//get rune
		runeIndex := indices[i]

		var ch *character
		//skip runes that are not in font chacter range
		if int(runeIndex)-int(lowChar) > len(f.fontChar) || runeIndex < lowChar {
			// print a ?
			ch = f.fontChar[int(rune('?'))-int(lowChar)]
		} else {
			// find rune in fontChar list
			ch = f.fontChar[runeIndex-lowChar]
		}

		//calculate position and size for current rune
		xpos := x + float32(ch.bearingH)*scale
		ypos := y - float32(ch.height-ch.bearingV)*scale
		w := float32(ch.width) * scale
		h := float32(ch.height) * scale

		//set quad positions
		var x1 = xpos
		var x2 = xpos + w
		var y1 = ypos
		var y2 = ypos + h

		coords = append(coords, point{x1, y1, float32(ch.x) / f.atlasWidth, float32(ch.y) / f.atlasHeight})
		coords = append(coords, point{x2, y1, float32(ch.x+ch.width) / f.atlasWidth, float32(ch.y) / f.atlasHeight})
		coords = append(coords, point{x1, y2, float32(ch.x) / f.atlasWidth, float32(ch.y+ch.height) / f.atlasHeight})
		coords = append(coords, point{x2, y1, float32(ch.x+ch.width) / f.atlasWidth, float32(ch.y) / f.atlasHeight})
		coords = append(coords, point{x1, y2, float32(ch.x) / f.atlasWidth, float32(ch.y+ch.height) / f.atlasHeight})
		coords = append(coords, point{x2, y2, float32(ch.x+ch.width) / f.atlasWidth, float32(ch.y+ch.height) / f.atlasHeight})

		// Now advance cursors for next glyph (note that advance is number of 1/64 pixels)
		x += float32((ch.advance >> 6)) * scale // Bitshift by 6 to get value in pixels (2^6 = 64 (divide amount of 1/64th pixels by 64 to get amount of pixels))
	}

	gl.BindVertexArray(f.vao)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, f.textureID)
	gl.BindBuffer(gl.ARRAY_BUFFER, f.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(coords)*16, gl.Ptr(coords), gl.DYNAMIC_DRAW)
	gl.DrawArrays(gl.TRIANGLES, 0, int32(len(coords)))
	gl.BindVertexArray(0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
	gl.Disable(gl.BLEND)
	gl.Disable(gl.SCISSOR_TEST)

	return nil
}

//Width returns the width of a piece of text in pixels
func (f *Font) Width(scale float32, fs string, argv ...interface{}) float32 {

	var width float32

	indices := []rune(fmt.Sprintf(fs, argv...))

	if len(indices) == 0 {
		return 0
	}

	lowChar := rune(32)

	// Iterate through all characters in string
	for i := range indices {

		//get rune
		runeIndex := indices[i]

		//skip runes that are not in font chacter range
		if int(runeIndex)-int(lowChar) > len(f.fontChar) || runeIndex < lowChar {
			fmt.Printf("%c %d\n", runeIndex, runeIndex)
			continue
		}

		//find rune in fontChar list
		ch := f.fontChar[runeIndex-lowChar]

		// Now advance cursors for next glyph (note that advance is number of 1/64 pixels)
		width += float32((ch.advance >> 6)) * scale // Bitshift by 6 to get value in pixels (2^6 = 64 (divide amount of 1/64th pixels by 64 to get amount of pixels))

	}

	return width
}
