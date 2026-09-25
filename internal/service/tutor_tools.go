package service

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// ========== Tipe protokol function calling ==========

type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolResult struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	OK     bool        `json:"ok"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

func propStr(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "string", "description": desc}
}

func propInt(desc string) map[string]interface{} {
	return map[string]interface{}{"type": "integer", "description": desc}
}

func schema(props map[string]interface{}, required ...string) map[string]interface{} {
	return map[string]interface{}{"type": "object", "properties": props, "required": required}
}

func GetToolDefinitions() []ToolDefinition {
	empty := schema(map[string]interface{}{})
	return []ToolDefinition{
		{Name: "set_nama_siswa", Description: "Simpan nama siswa yang disebutkan user.", Parameters: schema(map[string]interface{}{"nama": propStr("Nama siswa")}, "nama")},
		{Name: "set_kode_kelas", Description: "Simpan kode kelas yang disebutkan user. Backend menormalkan noise STT menjadi 6 digit DAN langsung mendaftarkan siswa (auto-join). JANGAN minta konfirmasi ulang kode setelah tool ini sukses.", Parameters: schema(map[string]interface{}{"kode": propStr("Kode kelas mentah dari ucapan user")}, "kode")},
		{Name: "join_kelas", Description: "Validasi kode tersimpan dan daftarkan siswa ke kelas. Hasil sukses/gagal adalah satu-satunya sumber kebenaran status join. Biasanya tidak perlu dipanggil manual karena set_kode_kelas sudah auto-join.", Parameters: empty},
		{Name: "get_kelas_info", Description: "Info kelas siswa: nama kelas, jumlah materi dan soal tersedia.", Parameters: empty},
		{Name: "list_materi", Description: "Daftar maksimal 10 materi di kelas siswa: id, judul, nama modul.", Parameters: empty},
		{Name: "get_materi", Description: "Ambil konten penuh satu materi untuk tanya-jawab.", Parameters: schema(map[string]interface{}{"materi_id": propInt("ID materi")}, "materi_id")},
		{Name: "pilih_materi", Description: "Tandai materi yang dipilih user dan siapkan sesi baca.", Parameters: schema(map[string]interface{}{"materi_id": propInt("ID materi")}, "materi_id")},
		{Name: "baca_bagian_pertama", Description: "Ambil paruh pertama konten materi terpilih untuk dibacakan.", Parameters: empty},
		{Name: "baca_bagian_kedua", Description: "Ambil paruh kedua konten materi terpilih (user sudah paham bagian pertama).", Parameters: empty},
		{Name: "ulang_baca", Description: "Ambil ulang paruh pertama konten (user belum paham).", Parameters: empty},
		{Name: "reset_percakapan", Description: "Hapus seluruh state session (mulai dari awal).", Parameters: empty},
		{Name: "list_soal", Description: "Ambil daftar jenis soal yang tersedia di kelas siswa beserta jumlahnya (harian/uts/uas).", Parameters: empty},
		{Name: "mulai_kuis", Description: "Mulai kuis suara dengan jenis tertentu. Kembalikan soal pertama lengkap dengan pilihan ganda.", Parameters: schema(map[string]interface{}{"jenis": propStr("jenis soal: harian, uts, atau uas")}, "jenis")},
		{Name: "ulang_soal", Description: "Bacakan ulang soal kuis yang sedang aktif.", Parameters: empty},
		{Name: "submit_jawaban", Description: "Nilai jawaban suara siswa untuk soal kuis aktif, simpan ke database, lalu lanjut ke soal berikutnya atau akhiri kuis dengan skor.", Parameters: schema(map[string]interface{}{"jawaban": propStr("jawaban siswa berupa huruf pilihan atau teks pilihan")}, "jawaban")},
	}
}

func (s *TutorService) ExecuteToolCalls(sessionID string, calls []ToolCall) []ToolResult {
	results := make([]ToolResult, 0, len(calls))
	for _, call := range calls {
		res := s.executeTool(sessionID, call)
		log.Printf("[tools] session=%s tool=%s ok=%v err=%q", sessionID, call.Name, res.OK, res.Error)
		results = append(results, res)
	}
	return results
}

func okResult(call ToolCall, result interface{}) ToolResult {
	return ToolResult{ID: call.ID, Name: call.Name, OK: true, Result: result}
}

func errResult(call ToolCall, msg string) ToolResult {
	return ToolResult{ID: call.ID, Name: call.Name, OK: false, Error: msg}
}

// tryJoinKelas mencoba daftarkan siswa pakai kode tersimpan.
// Mengubah state langsung kalau sukses. Return (ok, pesanError).
func (s *TutorService) tryJoinKelas(state *SessionData, save func()) (bool, string) {
	if state.Joined {
		return true, ""
	}
	if !kodeKelasValid(state.LastCode) {
		return false, "kode kelas belum valid atau belum disebutkan"
	}
	nama := state.Nama
	if nama == "" {
		nama = "Siswa"
	}
	siswa, token, err := s.siswaService.JoinSiswa(state.LastCode, nama)
	if err != nil {
		state.FailedCode = state.LastCode
		save()
		return false, err.Error()
	}
	namaKelas, _ := s.siswaService.NamaKelasByID(siswa.KelasID)
	konten := s.siswaService.HitungKontenKelas(siswa.KelasID)

	state.Joined = true
	state.KelasNama = namaKelas
	state.Nama = siswa.Nama
	state.KelasID = siswa.KelasID
	state.SiswaID = siswa.ID
	state.Konten = konten
	state.FailedCode = ""
	state.PendingJoin = &JoinInfo{
		SiswaID: siswa.ID, KelasID: siswa.KelasID, Nama: siswa.Nama,
		KelasNama: namaKelas, Token: token,
	}
	save()
	s.progressRepo.Touch(siswa.ID) // PROGRESS: aktivitas pertama
	return true, ""
}

func (s *TutorService) executeTool(sessionID string, call ToolCall) ToolResult {
	state := s.loadState(sessionID)
	save := func() { s.saveState(sessionID, state) }

	switch call.Name {

	case "set_nama_siswa":
		var args struct {
			Nama string `json:"nama"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return errResult(call, "arguments tidak valid")
		}
		nama := strings.TrimSpace(args.Nama)
		if nama == "" {
			return errResult(call, "nama kosong")
		}
		if r := []rune(nama); len(r) > 40 {
			nama = string(r[:40])
		}
		state.Nama = nama
		save()
		return okResult(call, map[string]string{"nama": nama})

	case "set_kode_kelas":
		var args struct {
			Kode string `json:"kode"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return errResult(call, "arguments tidak valid")
		}
		normalized := extractKodeKelas(args.Kode, state.LastCode)
		if normalized == "" {
			return errResult(call, "tidak ditemukan 6 digit angka pada kode")
		}
		state.LastCode = normalized
		save()

		// AUTO-JOIN: begitu kode valid, langsung daftarkan siswa.
		// Ini menghapus loop "benarkah kode ini?" yang bikin AI stuck.
		if !state.Joined {
			ok, errStr := s.tryJoinKelas(state, save)
			if ok {
				return okResult(call, map[string]interface{}{
					"kode":       normalized,
					"status":     "auto_joined",
					"siswa_id":   state.SiswaID,
					"nama":       state.Nama,
					"kelas_nama": state.KelasNama,
					"konten":     state.Konten,
					"instruksi":  "Siswa SUDAH terdaftar di kelas ini. JANGAN minta konfirmasi kode lagi. Sambut siswa ke kelas dan tawarkan: lihat daftar materi atau mulai kuis.",
				})
			}
			return okResult(call, map[string]interface{}{
				"kode":      normalized,
				"status":    "join_gagal",
				"error":     errStr,
				"instruksi": "Kode tidak ditemukan/tidak berlaku. Beri tahu siswa dengan ramah dan minta kode 6 digit yang lain.",
			})
		}
		return okResult(call, map[string]interface{}{"kode": normalized, "status": "sudah_join"})

	case "join_kelas":
		ok, errStr := s.tryJoinKelas(state, save)
		if !ok {
			return errResult(call, errStr)
		}
		return okResult(call, map[string]interface{}{
			"siswa_id": state.SiswaID, "nama": state.Nama, "kelas_nama": state.KelasNama, "konten": state.Konten,
		})

	case "get_kelas_info":
		if !state.Joined {
			return errResult(call, "siswa belum join kelas")
		}
		return okResult(call, map[string]interface{}{"kelas_nama": state.KelasNama, "konten": state.Konten})

	case "list_materi":
		if !state.Joined {
			return errResult(call, "siswa belum join kelas")
		}
		list, err := s.siswaService.GetDaftarMateri(state.KelasID)
		if err != nil {
			return errResult(call, err.Error())
		}
		ringkas := make([]map[string]interface{}, 0, 10)
		for i, m := range list {
			if i >= 10 {
				break
			}
			ringkas = append(ringkas, map[string]interface{}{"id": m["id"], "judul": m["judul"], "modul": m["modul"]})
		}
		return okResult(call, map[string]interface{}{"total": len(list), "materi": ringkas})

	case "get_materi":
		var args struct {
			MateriID uint `json:"materi_id"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return errResult(call, "arguments tidak valid")
		}
		materi, modulNama, err := s.siswaService.GetMateriByID(args.MateriID)
		if err != nil || materi == nil {
			return errResult(call, "materi tidak ditemukan")
		}
		return okResult(call, map[string]interface{}{"judul": materi.Judul, "modul": modulNama, "konten": materi.Konten})

	case "pilih_materi":
		var args struct {
			MateriID uint `json:"materi_id"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return errResult(call, "arguments tidak valid")
		}
		materi, modulNama, err := s.siswaService.GetMateriByID(args.MateriID)
		if err != nil || materi == nil {
			return errResult(call, "materi tidak ditemukan")
		}
		state.Belajar = &StateBelajar{
			Fase: "siap_baca", MateriID: materi.ID, MateriJudul: materi.Judul,
			MateriKonten: materi.Konten, ModulNama: modulNama, ProgressBaca: 0,
		}
		save()
		return okResult(call, map[string]interface{}{"judul": materi.Judul, "modul": modulNama})

	case "baca_bagian_pertama", "ulang_baca":
		if state.Belajar == nil || state.Belajar.MateriKonten == "" {
			return errResult(call, "belum ada materi yang dipilih")
		}
		first, _ := splitKonten(state.Belajar.MateriKonten)
		state.Belajar.Fase = "baca"
		state.Belajar.ProgressBaca = 50
		save()
		return okResult(call, map[string]interface{}{"judul": state.Belajar.MateriJudul, "bagian": first, "progress": 50})

	case "baca_bagian_kedua":
		if state.Belajar == nil || state.Belajar.MateriKonten == "" {
			return errResult(call, "belum ada materi yang dipilih")
		}
		_, second := splitKonten(state.Belajar.MateriKonten)
		materiID := state.Belajar.MateriID
		state.Belajar.Fase = "selesai"
		state.Belajar.ProgressBaca = 100
		save()
		s.progressRepo.SelesaikanMateri(state.SiswaID, materiID) // PROGRESS: materi selesai
		return okResult(call, map[string]interface{}{"judul": state.Belajar.MateriJudul, "bagian": second, "progress": 100})

	case "reset_percakapan":
		s.deleteState(sessionID)
		return okResult(call, map[string]string{"status": "session direset"})

	case "list_soal":
		if !state.Joined {
			return errResult(call, "siswa belum join kelas")
		}
		info := s.kuisService.InfoSoalKelas(state.KelasID)
		return okResult(call, map[string]interface{}{"tersedia": len(info) > 0, "info": info})

	case "mulai_kuis":
		var args struct {
			Jenis string `json:"jenis"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return errResult(call, "arguments tidak valid")
		}
		if !state.Joined {
			return errResult(call, "siswa belum join kelas")
		}
		jenis := strings.ToLower(strings.TrimSpace(args.Jenis))
		soals, err := s.kuisService.AmbilSoalKelas(state.KelasID, jenis, 5)
		if err != nil || len(soals) == 0 {
			return errResult(call, "tidak ada soal jenis "+jenis+" di kelas ini")
		}
		ids := make([]uint, 0, len(soals))
		for _, so := range soals {
			ids = append(ids, so.ID)
		}
		first := soals[0]
		state.Kuis = &StateKuis{
			Jenis: jenis, SoalIDs: ids, Index: 0, Skor: 0, Total: len(ids),
			SoalAktif: NewSoalAktif(1, len(ids), &first),
		}
		save()
		return okResult(call, map[string]interface{}{"jenis": jenis, "total": len(ids), "soal": FormatSoal(1, len(ids), &first)})

	case "ulang_soal":
		if state.Kuis == nil || state.Kuis.Index >= len(state.Kuis.SoalIDs) {
			return errResult(call, "tidak ada kuis aktif")
		}
		soal, err := s.kuisService.SoalByID(state.Kuis.SoalIDs[state.Kuis.Index])
		if err != nil {
			return errResult(call, "soal tidak ditemukan")
		}
		return okResult(call, map[string]interface{}{"soal": FormatSoal(state.Kuis.Index+1, state.Kuis.Total, soal)})

	case "submit_jawaban":
		var args struct {
			Jawaban string `json:"jawaban"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return errResult(call, "arguments tidak valid")
		}
		if state.Kuis == nil || state.Kuis.Index >= len(state.Kuis.SoalIDs) {
			return errResult(call, "tidak ada kuis aktif")
		}
		soal, err := s.kuisService.SoalByID(state.Kuis.SoalIDs[state.Kuis.Index])
		if err != nil {
			return errResult(call, "soal tidak ditemukan")
		}
		huruf := DeteksiHuruf(args.Jawaban, soal)
		if huruf == "" {
			return okResult(call, map[string]interface{}{
				"terdeteksi": false,
				"pesan":      "jawaban tidak terbaca sebagai pilihan a sampai d; tanyakan ulang ke siswa",
			})
		}
		benar := huruf == strings.ToUpper(strings.TrimSpace(soal.KunciJawaban))
		feedback := "Kurang tepat. Jawaban yang benar adalah " + strings.ToUpper(soal.KunciJawaban) + "."
		if benar {
			feedback = "Benar! Kerja bagus."
			state.Kuis.Skor++
		}
		if state.SiswaID > 0 {
			_ = s.kuisService.SimpanJawaban(state.SiswaID, soal.ID, args.Jawaban, huruf, benar, feedback)
			s.progressRepo.Touch(state.SiswaID) // PROGRESS: aktivitas kuis
		}

		state.Kuis.Index++
		skor := state.Kuis.Skor
		res := map[string]interface{}{"terdeteksi": true, "huruf": huruf, "benar": benar, "feedback": feedback, "skor": skor}
		if state.Kuis.Index >= state.Kuis.Total {
			res["selesai"] = true
			res["skor_akhir"] = fmt.Sprintf("%d dari %d", skor, state.Kuis.Total)
			state.Kuis = nil
		} else {
			next, errNext := s.kuisService.SoalByID(state.Kuis.SoalIDs[state.Kuis.Index])
			if errNext != nil {
				return errResult(call, "soal berikutnya tidak ditemukan")
			}
			state.Kuis.SoalAktif = NewSoalAktif(state.Kuis.Index+1, state.Kuis.Total, next)
			res["selesai"] = false
			res["soal_berikutnya"] = FormatSoal(state.Kuis.Index+1, state.Kuis.Total, next)
		}
		save()
		return okResult(call, res)

	default:
		return errResult(call, "tool tidak dikenal: "+call.Name)
	}
}

func splitKonten(konten string) (string, string) {
	r := []rune(konten)
	n := len(r)
	if n == 0 {
		return "", ""
	}
	cut := n / 2
	batasMin := int(float64(n) * 0.4)
	for i := cut; i >= batasMin; i-- {
		if r[i] == '\n' || r[i] == '.' || r[i] == '?' || r[i] == '!' {
			cut = i + 1
			break
		}
	}
	return strings.TrimSpace(string(r[:cut])), strings.TrimSpace(string(r[cut:]))
}