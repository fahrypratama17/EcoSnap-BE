package service

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/fahrypratama17/EcoSnap-BE/domain/entity"
	"github.com/go-pdf/fpdf"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

// The PDF embeds the Go font family (BSD licensed, from x/image), so text is
// never limited to the 14 built-in Latin-1 fonts: Indonesian and other Latin,
// Greek and Cyrillic names render exactly. Characters outside the font (for
// example CJK or emoji) cannot be drawn; they are replaced by a visible
// marker and the PDF says so in a note, never by a silent "?".

const (
	pdfFamily      = "gofont"
	pdfMargin      = 15.0
	pdfBottom      = 18.0
	pdfLine        = 5.5
	pdfPad         = 1.5
	pdfContentW    = 180.0
	pdfReplacement = '\uFFFD'
)

type fontCoverage struct {
	regular, bold *sfnt.Font
}

var (
	coverageOnce sync.Once
	coverage     fontCoverage
	coverageErr  error
)

func loadCoverage() (fontCoverage, error) {
	coverageOnce.Do(func() {
		if coverage.regular, coverageErr = sfnt.Parse(goregular.TTF); coverageErr != nil {
			return
		}
		coverage.bold, coverageErr = sfnt.Parse(gobold.TTF)
	})
	return coverage, coverageErr
}

func (c fontCoverage) has(r rune) bool {
	var b sfnt.Buffer
	for _, f := range []*sfnt.Font{c.regular, c.bold} {
		if i, err := f.GlyphIndex(&b, r); err != nil || i == 0 {
			return false
		}
	}
	return true
}

// pdfText turns user-controlled text into drawable text: control characters
// become spaces and glyphs missing from the embedded font become U+FFFD.
func (c fontCoverage) pdfText(s string, replaced *int) string {
	var out strings.Builder
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\t' || unicode.IsControl(r):
			out.WriteRune(' ')
		case c.has(r):
			out.WriteRune(r)
		default:
			*replaced++
			out.WriteRune(pdfReplacement)
		}
	}
	return out.String()
}

type pdfColumn struct {
	title string
	width float64
	align string
}

type pdfWriter struct {
	doc      *fpdf.Fpdf
	cov      fontCoverage
	replaced int
}

func (p *pdfWriter) text(s string) string { return p.cov.pdfText(s, &p.replaced) }

func (p *pdfWriter) setFont(style string, size float64) { p.doc.SetFont(pdfFamily, style, size) }

func (p *pdfWriter) ensure(height float64) {
	if p.doc.GetY()+height > 297-pdfBottom {
		p.doc.AddPage()
	}
}

// paragraph writes wrapped text; nothing is cut off.
func (p *pdfWriter) paragraph(s string, style string, size float64) {
	p.setFont(style, size)
	for _, line := range p.doc.SplitText(p.text(s), pdfContentW) {
		p.ensure(pdfLine)
		p.doc.CellFormat(pdfContentW, pdfLine, line, "", 1, "L", false, 0, "")
	}
}

func (p *pdfWriter) heading(s string) {
	p.ensure(pdfLine*3 + 2)
	p.doc.Ln(3)
	p.paragraph(s, "B", 12)
	p.doc.Ln(1)
}

// row draws one table row whose height fits its tallest wrapped cell and
// moves to a new page, repeating the header, when it does not fit.
func (p *pdfWriter) row(cols []pdfColumn, cells []string, header, bold bool) {
	style := ""
	if header || bold {
		style = "B"
	}
	p.setFont(style, 9.5)
	lines := make([][]string, len(cells))
	most := 1
	for i, c := range cells {
		lines[i] = p.doc.SplitText(p.text(c), cols[i].width-2*pdfPad)
		if len(lines[i]) == 0 {
			lines[i] = []string{""}
		}
		most = max(most, len(lines[i]))
	}
	h := float64(most)*pdfLine + 2
	if p.doc.GetY()+h > 297-pdfBottom {
		p.doc.AddPage()
		if !header {
			p.row(cols, titles(cols), true, false)
			p.setFont(style, 9.5)
		}
	}
	x, y := pdfMargin, p.doc.GetY()
	for i, col := range cols {
		if header {
			p.doc.SetFillColor(225, 240, 228)
			p.doc.Rect(x, y, col.width, h, "DF")
		} else {
			p.doc.Rect(x, y, col.width, h, "D")
		}
		for j, line := range lines[i] {
			p.doc.SetXY(x+pdfPad, y+1+float64(j)*pdfLine)
			p.doc.CellFormat(col.width-2*pdfPad, pdfLine, line, "", 0, col.align, false, 0, "")
		}
		x += col.width
	}
	p.doc.SetXY(pdfMargin, y+h)
}

func titles(cols []pdfColumn) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.title
	}
	return out
}

func (p *pdfWriter) table(cols []pdfColumn, rows [][]string, lastBold bool) {
	p.row(cols, titles(cols), true, false)
	for i, r := range rows {
		p.row(cols, r, false, lastBold && i == len(rows)-1)
	}
}

func int64Text(v *int64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprint(*v)
}

// RenderActivityReportPDF renders a report as a multi-page A4 PDF with the
// same period, totals, daily rows and categories as the JSON. It reads no
// state and stores nothing.
func RenderActivityReportPDF(r entity.ActivityReport) ([]byte, error) {
	cov, err := loadCoverage()
	if err != nil {
		return nil, err
	}
	doc := fpdf.New("P", "mm", "A4", "")
	doc.SetMargins(pdfMargin, pdfMargin, pdfMargin)
	doc.SetAutoPageBreak(false, 0)
	doc.AddUTF8FontFromBytes(pdfFamily, "", goregular.TTF)
	doc.AddUTF8FontFromBytes(pdfFamily, "B", gobold.TTF)
	doc.SetLang("id")
	doc.SetCreationDate(r.GeneratedAt)
	doc.AliasNbPages("{nb}")
	p := &pdfWriter{doc: doc, cov: cov}
	doc.SetTitle(p.text("Laporan Aktivitas EcoSnap"), true)
	doc.SetCreator("EcoSnap", true)
	doc.SetFooterFunc(func() {
		doc.SetY(-12)
		doc.SetFont(pdfFamily, "", 8)
		doc.CellFormat(0, 5, fmt.Sprintf("Halaman %d dari {nb}", doc.PageNo()), "", 0, "C", false, 0, "")
	})
	doc.AddPage()

	isUser := r.Account.Role == "user"
	roleName := "Mitra"
	if isUser {
		roleName = "Pengguna"
	}
	p.paragraph("Laporan Aktivitas EcoSnap", "B", 16)
	p.doc.Ln(1)
	p.paragraph("Akun: "+r.Account.FullName+" ("+roleName+")", "", 10)
	if r.Account.BusinessName != nil {
		p.paragraph("Lokasi mitra: "+*r.Account.BusinessName, "", 10)
	}
	const stamp = "2006-01-02 15:04:05 -07:00"
	p.paragraph("Periode: "+r.Period.From.Format(stamp)+" sampai "+r.Period.To.Format(stamp)+" (awal termasuk, akhir tidak termasuk)", "", 10)
	p.paragraph(fmt.Sprintf("Zona waktu rekap harian: %s (WIB, UTC+7); %d tanggal. Hari pertama dan terakhir dapat sebagian.", r.Period.Timezone, r.Period.Days), "", 10)
	p.paragraph("Dibuat: "+r.GeneratedAt.In(r.Period.From.Location()).Format(stamp), "", 10)
	if isUser {
		p.paragraph("Dihitung dari setoran yang Anda klaim, menurut waktu klaim. Poin diperoleh dan poin dipakai dilaporkan terpisah; saldo dompet bukan bagian laporan ini.", "", 9)
	} else {
		p.paragraph("Dihitung dari setoran lokasi Anda yang sudah diklaim, menurut waktu klaim. Setoran yang belum diklaim tidak dihitung dan identitas pengklaim tidak ditampilkan.", "", 9)
	}

	p.heading("Ringkasan")
	summary := [][]string{
		{"Setoran diklaim", fmt.Sprint(r.Totals.ClaimedDeposits)},
		{"Berat total (kg)", r.Totals.TotalWeightKG},
	}
	if isUser {
		summary = append(summary, []string{"Poin diperoleh", int64Text(r.Totals.PointsEarned)}, []string{"Poin dipakai (penukaran)", int64Text(r.Totals.PointsSpent)})
	} else {
		summary = append(summary, []string{"Poin dikreditkan ke pengguna", int64Text(r.Totals.PointsCredited)})
	}
	p.table([]pdfColumn{{"Keterangan", 110, "L"}, {"Nilai", 70, "R"}}, summary, false)

	p.heading("Rincian harian (WIB)")
	var cols []pdfColumn
	var rows [][]string
	if isUser {
		cols = []pdfColumn{{"Tanggal", 42, "L"}, {"Setoran", 28, "R"}, {"Berat (kg)", 38, "R"}, {"Poin diperoleh", 38, "R"}, {"Poin dipakai", 34, "R"}}
		for _, d := range r.Daily {
			rows = append(rows, []string{d.Date, fmt.Sprint(d.ClaimedDeposits), d.TotalWeightKG, int64Text(d.PointsEarned), int64Text(d.PointsSpent)})
		}
		rows = append(rows, []string{"Jumlah", fmt.Sprint(r.Totals.ClaimedDeposits), r.Totals.TotalWeightKG, int64Text(r.Totals.PointsEarned), int64Text(r.Totals.PointsSpent)})
	} else {
		cols = []pdfColumn{{"Tanggal", 50, "L"}, {"Setoran", 30, "R"}, {"Berat (kg)", 45, "R"}, {"Poin dikreditkan", 55, "R"}}
		for _, d := range r.Daily {
			rows = append(rows, []string{d.Date, fmt.Sprint(d.ClaimedDeposits), d.TotalWeightKG, int64Text(d.PointsCredited)})
		}
		rows = append(rows, []string{"Jumlah", fmt.Sprint(r.Totals.ClaimedDeposits), r.Totals.TotalWeightKG, int64Text(r.Totals.PointsCredited)})
	}
	p.table(cols, rows, true)

	p.heading("Rincian per kategori")
	if len(r.Categories) == 0 {
		p.paragraph("Tidak ada setoran diklaim pada periode ini.", "", 10)
	} else {
		var crows [][]string
		for _, c := range r.Categories {
			crows = append(crows, []string{c.CategoryName, fmt.Sprint(c.DepositCount), c.TotalWeightKG})
		}
		p.table([]pdfColumn{{"Kategori", 100, "L"}, {"Setoran", 35, "R"}, {"Berat (kg)", 45, "R"}}, crows, false)
		p.paragraph("Nama kategori diambil dari catatan setoran saat dibuat. Satu setoran dapat memuat beberapa kategori, sehingga jumlah setoran per kategori tidak dijumlahkan.", "", 9)
	}
	if p.replaced > 0 {
		p.doc.Ln(2)
		p.paragraph(fmt.Sprintf("Catatan: %d karakter tidak tersedia pada font dokumen ini dan ditandai dengan %c.", p.replaced, pdfReplacement), "", 9)
	}
	if !doc.Ok() {
		return nil, doc.Error()
	}
	var buf bytes.Buffer
	if err := doc.Output(&buf); err != nil {
		return nil, err
	}
	if buf.Len() == 0 {
		return nil, errors.New("PDF kosong")
	}
	return buf.Bytes(), nil
}
