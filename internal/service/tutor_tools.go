package service

import (
	"encoding/json"
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
	return map[string]interface{}{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

// GetToolDefinitions daftar kemampuan backend yang boleh dipanggil AI
func GetToolDefinitions() []ToolDefinition {
	empty := schema(map[string]interface{}{})
	return []ToolDefinition{
		{Name: "set_nama_siswa", Description: "Simpan nama siswa yang disebutkan user.", Parameters: schema(map[string]interface{}{"nama": propStr("Nama siswa")}, "nama")},
		{Name: "set_kode_kelas", Description: "Simpan kode kelas yang disebutkan user. Backend menormalkan noise STT menjadi 6 digit.", Parameters: schema(map[string]interface{}{"kode": propStr("Kode kelas mentah dari ucapan user")}, "kode")},
		{Name: "join_kelas", Description: "Validasi kode tersimpan dan daftarkan siswa ke kelas. Hasil sukses/gagal adalah satu-satunya sumber kebenaran status join.", Parameters: empty},
		{Name: "get_kelas_info", Description: "Info kelas siswa: nama kelas, jumlah materi dan soal tersedia.", Parameters: empty},
		{Name: "list_materi", Description: "Daftar maksimal 10 materi di kelas siswa: id, judul, nama modul.", Parameters: empty},
		{Name: "get_materi", Description: "Ambil konten penuh satu materi untuk tanya-jawab.", Parameters: schema(map[string]interface{}{"materi_id": propInt("ID materi")}, "materi_id")},
		{Name: "pilih_materi", Description: "Tandai materi yang dipilih user dan siapkan sesi baca.", Parameters: schema(map[string]interface{}{"materi_id": propInt("ID materi")}, "materi_id")},
		{Name: "baca_bagian_pertama", Description: "Ambil paruh pertama konten materi terpilih untuk dibacakan.", Parameters: empty},
		{Name: "baca_bagian_kedua", Description: "Ambil paruh kedua konten materi terpilih (user sudah paham bagian pertama).", Parameters: empty},
		{Name: "ulang_baca", Description: "Ambil ulang paruh pertama konten (user belum paham).", Parameters: empty},
		{Name: "reset_percakapan", Description: "Hapus seluruh state session (mulai dari awal).", Parameters: empty},
	}
}

// ExecuteToolCalls dipakai handler endpoint internal tools
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

func (s *TutorService) executeTool(sessionID string, call ToolCall) ToolResult {
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
		s.mu.Lock()
		s.sessionNama[sessionID] = nama
		s.mu.Unlock()
		return okResult(call, map[string]string{"nama": nama})

	case "set_kode_kelas":
		var args struct {
			Kode string `json:"kode"`
		}
		if err := json.Unmarshal(call.Arguments, &args); err != nil {
			return errResult(call, "arguments tidak valid")
		}
		s.mu.Lock()
		prev := s.sessionLastCode[sessionID]
		s.mu.Unlock()
		normalized := extractKodeKelas(args.Kode, prev)
		if normalized == "" {
			return errResult(call, "tidak ditemukan 6 digit angka pada kode")
		}
		s.mu.Lock()
		s.sessionLastCode[sessionID] = normalized
		s.mu.Unlock()
		return okResult(call, map[string]string{"kode": normalized})

	case "join_kelas":
		s.mu.Lock()
		kode := s.sessionLastCode[sessionID]
		nama := s.sessionNama[sessionID]
		joined := s.sessionJoined[sessionID]
		s.mu.Unlock()

		if joined {
			return okResult(call, map[string]interface{}{"sudah_join": true})
		}
		if !kodeKelasValid(kode) {
			return errResult(call, "kode kelas belum valid atau belum disebutkan")
		}
		if nama == "" {
			nama = "Siswa"
		}

		siswa, token, err := s.siswaService.JoinSiswa(kode, nama)
		if err != nil {
			s.mu.Lock()
			s.sessionFailedCode[sessionID] = kode
			s.mu.Unlock()
			return errResult(call, err.Error())
		}

		namaKelas, _ := s.siswaService.NamaKelasByID(siswa.KelasID)
		konten := s.siswaService.HitungKontenKelas(siswa.KelasID)

		s.mu.Lock()
		s.sessionJoined[sessionID] = true
		s.sessionKelasNama[sessionID] = namaKelas
		s.sessionNama[sessionID] = siswa.Nama
		s.sessionKelasID[sessionID] = siswa.KelasID
		s.sessionKonten[sessionID] = konten
		delete(s.sessionFailedCode, sessionID)
		// token disimpan di backend saja, dikirim ke FE lewat response /chat
		s.sessionPendingJoin[sessionID] = &JoinInfo{
			SiswaID:   siswa.ID,
			KelasID:   siswa.KelasID,
			Nama:      siswa.Nama,
			KelasNama: namaKelas,
			Token:     token,
		}
		s.mu.Unlock()

		return okResult(call, map[string]interface{}{
			"siswa_id":   siswa.ID,
			"nama":       siswa.Nama,
			"kelas_nama": namaKelas,
			"konten":     konten,
		})

	case "get_kelas_info":
		s.mu.Lock()
		kelasNama := s.sessionKelasNama[sessionID]
		konten := s.sessionKonten[sessionID]
		joined := s.sessionJoined[sessionID]
		s.mu.Unlock()
		if !joined {
			return errResult(call, "siswa belum join kelas")
		}
		return okResult(call, map[string]interface{}{"kelas_nama": kelasNama, "konten": konten})

	case "list_materi":
		s.mu.Lock()
		kelasID := s.sessionKelasID[sessionID]
		joined := s.sessionJoined[sessionID]
		s.mu.Unlock()
		if !joined {
			return errResult(call, "siswa belum join kelas")
		}
		list, err := s.siswaService.GetDaftarMateri(kelasID)
		if err != nil {
			return errResult(call, err.Error())
		}
		ringkas := make([]map[string]interface{}, 0, len(list))
		for i, m := range list {
			if i >= 10 {
				break
			}
			ringkas = append(ringkas, map[string]interface{}{
				"id":    m["id"],
				"judul": m["judul"],
				"modul": m["modul"],
			})
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
		return okResult(call, map[string]interface{}{
			"judul":  materi.Judul,
			"modul":  modulNama,
			"konten": materi.Konten,
		})

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
		s.mu.Lock()
		s.sessionBelajar[sessionID] = &StateBelajar{
			Fase:         "siap_baca",
			MateriID:     materi.ID,
			MateriJudul:  materi.Judul,
			MateriKonten: materi.Konten,
			ModulNama:    modulNama,
			ProgressBaca: 0,
		}
		s.mu.Unlock()
		return okResult(call, map[string]interface{}{
			"judul":           materi.Judul,
			"modul":           modulNama,
			"jumlah_karakter": len([]rune(materi.Konten)),
		})

	case "baca_bagian_pertama", "ulang_baca":
		s.mu.Lock()
		st := s.sessionBelajar[sessionID]
		if st == nil || st.MateriKonten == "" {
			s.mu.Unlock()
			return errResult(call, "belum ada materi yang dipilih")
		}
		first, _ := splitKonten(st.MateriKonten)
		st.Fase = "baca"
		st.ProgressBaca = 50
		s.mu.Unlock()
		return okResult(call, map[string]interface{}{"judul": st.MateriJudul, "bagian": first, "progress": 50})

	case "baca_bagian_kedua":
		s.mu.Lock()
		st := s.sessionBelajar[sessionID]
		if st == nil || st.MateriKonten == "" {
			s.mu.Unlock()
			return errResult(call, "belum ada materi yang dipilih")
		}
		_, second := splitKonten(st.MateriKonten)
		st.Fase = "selesai"
		st.ProgressBaca = 100
		s.mu.Unlock()
		return okResult(call, map[string]interface{}{"judul": st.MateriJudul, "bagian": second, "progress": 100})

	case "reset_percakapan":
		s.mu.Lock()
		delete(s.sessionLastCode, sessionID)
		delete(s.sessionFailedCode, sessionID)
		delete(s.sessionJoined, sessionID)
		delete(s.sessionKelasNama, sessionID)
		delete(s.sessionNama, sessionID)
		delete(s.sessionKelasID, sessionID)
		delete(s.sessionKonten, sessionID)
		delete(s.sessionBelajar, sessionID)
		delete(s.sessionPendingJoin, sessionID)
		s.mu.Unlock()
		return okResult(call, map[string]string{"status": "session direset"})

	default:
		return errResult(call, "tool tidak dikenal: "+call.Name)
	}
}

// splitKonten membelah konten di batas kalimat terdekat dari titik tengah
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