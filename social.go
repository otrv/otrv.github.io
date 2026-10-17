package main

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"path/filepath"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	socialWidth         = 1200
	socialHeight        = 630
	socialMargin        = 80
	socialTextWidth     = socialWidth - 2*socialMargin
	socialTitleLinesMax = 3
	socialTitleSizeMax  = 56.0
	socialTitleSizeMin  = 20.0
	socialFontBytesMax  = 8 << 20
)

func generateSocialImages(dir string, posts []Post) error {
	data, err := readFileLimited(filepath.Join(dir, "ibm-plex-mono-regular.ttf"), socialFontBytesMax)
	if err != nil {
		return fmt.Errorf("read social image font: %w", err)
	}
	typeface, err := opentype.Parse(data)
	if err != nil {
		return fmt.Errorf("parse social image font: %w", err)
	}
	if err := writeOutput(filepath.Join(dir, "social-index.png"), func(w io.Writer) error {
		return renderSocialImage(w, typeface, "Özgür Tanrıverdi", "Software engineer in Istanbul.")
	}); err != nil {
		return err
	}
	for _, post := range posts {
		if err := writeOutput(filepath.Join(dir, post.SocialImage()), func(w io.Writer) error {
			return renderSocialImage(w, typeface, post.Title, "Özgür Tanrıverdi / "+post.DateString())
		}); err != nil {
			return err
		}
	}
	return nil
}

func renderSocialImage(w io.Writer, typeface *opentype.Font, title, subtitle string) (err error) {
	canvas := image.NewRGBA(image.Rect(0, 0, socialWidth, socialHeight))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{0x1c, 0x1b, 0x19, 0xff}), image.Point{}, draw.Src)
	ink := image.NewUniform(color.RGBA{0xe8, 0xe3, 0xda, 0xff})
	sand := image.NewUniform(color.RGBA{0xc7, 0xb5, 0x8d, 0xff})
	muted := image.NewUniform(color.RGBA{0xa6, 0xa0, 0x96, 0xff})

	var titleFace font.Face
	var lines []string
	var lineHeight int
	for size := socialTitleSizeMax; size >= socialTitleSizeMin; size -= 2 {
		face, err := opentype.NewFace(typeface, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingNone})
		if err != nil {
			return err
		}
		lines = nil
		line := ""
		fits := true
		for _, word := range strings.Fields(title) {
			if font.MeasureString(face, word).Ceil() > socialTextWidth {
				fits = false
				break
			}
			candidate := strings.TrimSpace(line + " " + word)
			if font.MeasureString(face, candidate).Ceil() > socialTextWidth {
				lines = append(lines, line)
				line = word
			} else {
				line = candidate
			}
		}
		lines = append(lines, line)
		if fits && len(lines) <= socialTitleLinesMax {
			titleFace = face
			lineHeight = int(size * 1.3)
			break
		}
		if err := face.Close(); err != nil {
			return err
		}
	}
	if titleFace == nil {
		return fmt.Errorf("title is too long for social image: %s", title)
	}
	defer func() {
		err = errors.Join(err, titleFace.Close())
	}()
	if len(lines) == 2 {
		words := strings.Fields(title)
		bestWidth := socialTextWidth + 1
		for i := 1; i < len(words); i++ {
			left, right := strings.Join(words[:i], " "), strings.Join(words[i:], " ")
			width := max(font.MeasureString(titleFace, left).Ceil(), font.MeasureString(titleFace, right).Ceil())
			if width < bestWidth {
				lines = []string{left, right}
				bestWidth = width
			}
		}
	}
	labelFace, err := opentype.NewFace(typeface, &opentype.FaceOptions{Size: 26, DPI: 72, Hinting: font.HintingNone})
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, labelFace.Close())
	}()
	drawer := font.Drawer{Dst: canvas, Src: sand, Face: labelFace, Dot: fixed.P(socialMargin, 100)}
	drawer.DrawString("otrv.dev")
	drawer.Face = titleFace
	drawer.Src = ink
	for i, line := range lines {
		drawer.Dot = fixed.P(socialMargin, 330-(len(lines)-1)*lineHeight/2+i*lineHeight)
		drawer.DrawString(line)
	}
	drawer.Face = labelFace
	drawer.Src = muted
	drawer.Dot = fixed.P(socialMargin, 530)
	drawer.DrawString(subtitle)
	return png.Encode(w, canvas)
}
