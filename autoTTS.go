package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/fatih/color"
)

// Colors
var (
    bold_red   = color.New(color.FgRed, color.Bold)
    bold_green = color.New(color.FgGreen, color.Bold)
    bold_white = color.New(color.FgWhite, color.Bold)
	bold_cyan  = color.New(color.FgCyan, color.Bold)
)

func handleErr(process, error_message string, err error) {
	bold_red.Printf("[!] Error occured during %s\n", process)
	bold_red.Println("message:", error_message)
	bold_red.Println("more info:", err)
}

type AvailableVoices struct {
	voices []Voice
}

type Voice struct {
	ID   string
	Name string
}

func (Av *AvailableVoices) UpdateVoices() error {
	resp, err := http.Get("https://api.elevenlabs.io/v1/voices")
	if err != nil {
		handleErr("updating voices", "GET request failed", err)
		return err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		handleErr("updating voices", "failed to read response body", err)
		return err
	}

	var data map[string]interface{}
	var output []Voice

	err = json.Unmarshal(body, &data)
	if err != nil {
		handleErr("updating voices", "failed to unmarshal json", err)
		return err
	}
	voices := data["voices"].([]interface{})

	for _, v := range voices {
		voice := v.(map[string]interface{})
		output = append(output, Voice{
			ID:   voice["voice_id"].(string),
			Name: voice["name"].(string),
		})
	}

	Av.voices = output
	return nil
}

func (Av *AvailableVoices) VoiceByName(name string) (bool, Voice) {
	for _, voice := range Av.voices {
		if voice.Name == name {
			return true, voice
		}
	}
	return false, Voice{}
}

func (Av *AvailableVoices) VoiceByID(ID string) (bool, Voice) {
	for _, voice := range Av.voices {
		if voice.ID == ID {
			return true, voice
		}
	}
	return false, Voice{}
}


type Config struct {
	Api_Key           	string 				`json:"api-key"`
	Stability         	float32 			`json:"stability"`
	Similarity_Boost  	float32 			`json:"similarity-boost"`
	Style             	int 				`json:"style"`
	Use_Speaker_Boost 	bool				`json:"use-speaker-boost"`
	Voices 				map[string]string 	`json:"voices"`
}

func ParseConfig() (Config, error) {
	file, err := os.Open("config.json")
	if err != nil {
		handleErr("parsing config file", "failed to open config file", err)
		return Config{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)

	var config Config
	if err := decoder.Decode(&config); err != nil {
		handleErr("parsing config file", "failed to parse json", err)
		return Config{}, err
	}

	return config, nil
}

func GenerateConfig(api_key string) error {
	config := Config{
		Api_Key: api_key,
		Stability: 0.5,
		Similarity_Boost: 0,
		Style: 0,
		Use_Speaker_Boost: true,
		Voices: map[string]string{
			"Speaker1": "ErXwobaYiN019PkySvjV",
			"Speaker2": "VR6AewLTigWG4xSOukaG",
		},
	}

	configJSON, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return err
	}

	return ioutil.WriteFile("config.json", configJSON, os.ModePerm)
}


// Json
type json_tts struct { // text to speech
	Text 			string 	`json:"text"`
	Model_Id 		string 	`json:"model_id"`
	Voice_Settings 	json_vs `json:"voice_settings"`
}

type json_vs struct { // voice settings
	Stability 			float32 	`json:"stability"`
	Similarity_Boost 	float32 	`json:"similarity_boost"`
	Style 				int 		`json:"Style"`
	Use_Speaker_Boost 	bool 		`json:"use_speaker_boost"`
}

type json_account struct { // account
	Tier 								string 			`josn:"tier"`
	Character_Count 					int 			`json:"character_count"`
	Character_Limit 					int 			`json:"character_limit"`
	Can_Extend_Character_Limit 			bool 			`json:"can_extend_character_limit"`
	Allowed_To_Extend_Character_limit 	bool 			`json:"allowed_to_extend_character_limit"`
	Next_Character_Count_Reset_Unix 	int 			`json:"next_character_count_reset_unix"`
	Voice_Limit 						int 			`json:"voice_limit"`
	Max_Voice_Add_Edits 				int 			`json:"max_voice_add_edits"`
	Voice_Add_Edit_Counter 				int 			`json:"voice_add_edit_counter"`
	Professional_Voice_Limit 			int 			`json:"professional_voice_limit"`
	Can_Extend_Voice_limit 				bool 			`json:"can_extend_voice_limit"`
	Can_Use_Instant_Voice_Cloning 		bool 			`json:"can_use_instant_voice_cloning"`
	Can_Use_Professional_Voice_Cloning 	bool 			`json:"can_use_professional_voice_cloning"`
	Currency 							interface{} 	`json:"currency"`
	Status 								string 			`json:"status"`
	Next_Invoice 						interface{} 	`json:"next_invoice"`
	Has_Open_Invoices 					bool 			`json:"has_open_invoices"`
}

type AutoTTS struct {
	client 					*http.Client
	model_id 				string
	api_key 				string
	voices 					map[string]string
	script 					string
	tokens_per_character 	int
	config 					Config
}

type TTS struct {
	speaker 	string
	text 		string
	line 		int
}


func (autoTTS *AutoTTS) ReadScript() []TTS {
	output := []TTS{}
	for i, line := range strings.Split(autoTTS.script, "\n") {
		speaker := regexp.MustCompile(`^(.*):(.*)`)
		match := speaker.FindStringSubmatch(line)
		
		// match[1] is the speaker
		// match[2] is the tts

		if match != nil {
			match[2] = strings.TrimSpace(match[2])
			output = append(output, TTS{
				speaker: match[1],
				text: match[2],
				line: i+1,
			})
		}
	}
	return output
}

func (autoTTS *AutoTTS) Generate(text, voice_id string, voice_settings json_vs) ([]byte, error) {
    json_data, err := json.Marshal(
		json_tts{
			Text: text,
			Model_Id: autoTTS.model_id,
			Voice_Settings: voice_settings,
		},
	)
    if err != nil {
		handleErr("generating tts", "failed to marshal json", err)
		return []byte{}, err
	}
	
    url := fmt.Sprint("https://api.elevenlabs.io/v1/text-to-speech/", voice_id)
    req, err := http.NewRequest("POST", url, bytes.NewBuffer(json_data))
    if err != nil {
		handleErr("generating tts", "failed to create POST request", err)
		return []byte{}, err
	}

	req.Header.Add("accept", "*/*")
	req.Header.Add("xi-api-key", autoTTS.api_key)
	req.Header.Add("Content-Type", "application/json")

    resp, err := autoTTS.client.Do(req)
	if err != nil {
		handleErr("generating tts", "POST request failed", err)
		return []byte{}, err
	}
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			handleErr("generating tts", "failed to read request body", err)
			return []byte{}, err
		}
		return body, nil
    }

	body_bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		handleErr("getting account tokens", fmt.Sprintf("GET request not OK and failed to read response body, status code: %d", resp.StatusCode), err)
		return []byte{}, err
	}
	body := string(body_bytes)

	more_info := fmt.Errorf(body)
	if resp.StatusCode == 404 {
		more_info = fmt.Errorf("try copying script file contents into another file")
	}

	handleErr("generating tts", fmt.Sprintf("POST request not OK, status code: %d", resp.StatusCode), more_info)
	return []byte{}, fmt.Errorf("status: %d", resp.StatusCode)
}

func (autoTTS *AutoTTS) GetAccountTokens() (int, error) {
    req, err := http.NewRequest("GET", "https://api.elevenlabs.io/v1/user/subscription", nil)
    if err != nil {
		handleErr("getting account tokens", "failed to create GET request", err)
		return 0, err
	}

	req.Header.Add("accept", "*/*")
	req.Header.Add("xi-api-key", autoTTS.api_key)
	req.Header.Add("Content-Type", "application/json")

    resp, err := autoTTS.client.Do(req)
	if err != nil {
		handleErr("getting account tokens", "GET request failed", err)
		return 0, err
	}
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusOK {
		var json_resp json_account
		json.NewDecoder(resp.Body).Decode(&json_resp)
		return json_resp.Character_Limit - json_resp.Character_Count, nil
    }

	body_bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		handleErr("getting account tokens", fmt.Sprintf("GET request not OK and failed to read response body, status code: %d", resp.StatusCode), err)
		return 0, err
	}
	body := string(body_bytes)
	
	handleErr("getting account tokens", fmt.Sprintf("GET request not OK, status code: %d", resp.StatusCode), fmt.Errorf(body))
	return 0, fmt.Errorf("status: %d", resp.StatusCode)
}

func (autoTTS *AutoTTS) CalculateScriptCost() int {
	var cost int
	r := regexp.MustCompile(`.*:(.*)`)
	for _, m := range r.FindAllStringSubmatch(autoTTS.script, -1) {
		// length of word * autoTTS.tokens_per_character
		cost += len(m[1]) * autoTTS.tokens_per_character
	}
	return cost
}

func (autoTTS *AutoTTS) CalculateTTSCost(tts string) int {
	return len(tts) * autoTTS.tokens_per_character
}

func (autoTTS *AutoTTS) PlayAudio(audio_file_path string) error {
	dir, err := os.Getwd()
	if err != nil {
		handleErr("playing audio file", "failed to get current directory", err)
		return err
	}
	file_path := filepath.Join(dir, audio_file_path)
	
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
    case "windows":
        cmd = exec.Command("cmd", "/c", "start", file_path)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
    case "linux":
        cmd = exec.Command("xdg-open", file_path)
    case "darwin":
        cmd = exec.Command("open", file_path)
    default:
        return fmt.Errorf("unsupported platform")
    }

    return cmd.Run()
}


func GetScript() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		handleErr("getting script file", "failed to get current directory", err)
		return "", err
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		handleErr("getting script file", "failed to read directory", err)
		return "", err
	}

	for _, file := range files {
		if file.IsDir() {continue}
		if filepath.Ext(file.Name()) == ".txt" {
			return file.Name(), nil
		}
	}

	handleErr("getting script file", "script file not found", err)
	return "", fmt.Errorf("not found")
}

func ReceiveInput(prompt string) string {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print(prompt)
	scanner.Scan()
	return scanner.Text()
}


func main() {
	bold_cyan.Println(`╔═╗┬ ┬┌┬┐┌─┐  ╔╦╗╔╦╗╔═╗
╠═╣│ │ │ │ │   ║  ║ ╚═╗
╩ ╩└─┘ ┴ └─┘   ╩  ╩ ╚═╝
by Niu
 `)
	
	Voices := AvailableVoices{voices: []Voice{}}
	err := Voices.UpdateVoices()
	if err != nil {
		fmt.Println("Press Enter to exit...")
        fmt.Scanln()
		return
	}
	
	config, err := ParseConfig()
	if err != nil {
		fmt.Println("[*] Generating new config file")
		
		err = GenerateConfig(ReceiveInput("[?] API key> "))
		if err != nil {
			fmt.Println("[!] Failed to generate config file")
			fmt.Println("Press Enter to exit...")
        	fmt.Scanln()
			return
		}
		
		bold_green.Println("[+] Config file generated, edit appropriately and relaunch")
		return
	}

	// Convert voice name to ID
	for speaker, voice := range config.Voices {
		ID_exists, _ := Voices.VoiceByID(voice)
		if !ID_exists {
			name_exists, v := Voices.VoiceByName(voice)
			if name_exists {
				config.Voices[speaker] = v.ID
			} else {
				handleErr("loading voices", fmt.Sprint("invalid voice ID / name: ", voice), nil)
			}
		}
	}
	
	fmt.Println("[+] Config loaded")
	
	script_file, err := GetScript()
	if err != nil {
		fmt.Println("Press Enter to exit...")
        fmt.Scanln()
		return
	}
	
	start_from_input := ReceiveInput("[?] Start from (enter for 1)> ")
	start_from, err := strconv.Atoi(start_from_input)
	if err != nil {
		if start_from_input == "" {
			start_from = 1
		} else {
			bold_red.Println("[!] Needs to be a number")
			fmt.Println("Press Enter to exit...")
        	fmt.Scanln()
			return
		}
	}
	if start_from <= 0 {
		bold_red.Println("[!] Needs to be more than 0")
		fmt.Println("Press Enter to exit...")
        fmt.Scanln()
		return
	}
	
	script_bytes, err := ioutil.ReadFile(script_file)
	if err != nil {
		fmt.Println("Press Enter to exit...")
        fmt.Scanln()
		return
	}
	script := string(script_bytes)

	total_lines := len(strings.Split(script, "\n"))
	if start_from > total_lines {
		handleErr(
			"setting up",
			"starting line is higher than the total number of lines in script file",
			fmt.Errorf(fmt.Sprint("total number of lines is ", total_lines)),
		)
		fmt.Println("Press Enter to exit...")
        fmt.Scanln()
		return
	}
	
	autoTTS := AutoTTS{
		client: &http.Client{Timeout: 15 * time.Second},
		model_id: "eleven_monolingual_v1",
		api_key: config.Api_Key,
		voices: config.Voices,
		script: script,
		tokens_per_character: 1,
		config: config,
	}

	tts_directory := script_file[:len(script_file)-4]+"_tts"
	tts_directory = strings.ReplaceAll(tts_directory, " ", "_")
	
	if _, err := os.Stat(tts_directory); os.IsNotExist(err) {
		err = os.Mkdir(tts_directory, os.ModePerm)
		if err != nil {
			fmt.Println("[!] Couldn't create TTS directory")
			fmt.Println("Press Enter to exit...")
        	fmt.Scanln()
			return
		}
	}
	
	voice_settings := json_vs{
		Stability: autoTTS.config.Stability,
		Similarity_Boost: autoTTS.config.Similarity_Boost,
		Style: autoTTS.config.Style,
		Use_Speaker_Boost: autoTTS.config.Use_Speaker_Boost,
	}
	
	tokens, err := autoTTS.GetAccountTokens()
	if err != nil {
		fmt.Println("Press Enter to exit...")
        fmt.Scanln()
		return
	}
	cost := autoTTS.CalculateScriptCost()
	
	fmt.Printf(`
Loaded file: 	    %s
Starting from line: %s
Available tokens:   %s
Cost: 		    %s
`,
		bold_green.Sprint(script_file),
		bold_green.Sprint(start_from),
		bold_green.Sprint(tokens), 
		bold_green.Sprint(cost),
	)

	if tokens < cost { // Warn user
		resp := ReceiveInput("[!] Not enough tokens to generate full script, continue? (y/n): ")
		if resp == "n" {return}
	}

	for i_TTS, TTS := range autoTTS.ReadScript()[start_from-1:] {
		for { 
			index := i_TTS+start_from
			fmt.Printf("\nLine: %s | Speaker: %s\n", bold_green.Sprint(index), bold_green.Sprint(TTS.speaker))
			fmt.Println(TTS.text + "\n")
		
			tokens, err := autoTTS.GetAccountTokens()
			if err != nil {
				fmt.Println("Press Enter to exit...")
        		fmt.Scanln()
				return
			}
			cost := autoTTS.CalculateTTSCost(TTS.text)

			if tokens < cost {
				fmt.Println("[!] Not enough tokens, continue from line:", index)
				fmt.Println("Press Enter to exit...")
       			fmt.Scanln()
				return
			}

			voice := autoTTS.voices[TTS.speaker]
			if voice == "" {
				handleErr("generating tts", "speaker not in config", nil)
				fmt.Println("Press Enter to exit...")
        		fmt.Scanln()
				return
			}

			generate_prompt := fmt.Sprintf(
				"[?] Generate? [%s] %s | [%s] %s | [%s] %s> ",
				bold_green.Sprint("Y"),
				bold_green.Sprint("yes"),
				bold_red.Sprint("N"),
				bold_red.Sprint("no"),
				bold_white.Sprint("E"),
				bold_white.Sprint("edit"),
			)

			resp := ReceiveInput(generate_prompt)
			resp = strings.ToLower(resp)
			if resp == "n" {break}
			if resp == "e" {
				new_tts := ReceiveInput("[?] New TTS> ")
				TTS.text = new_tts
			}
			
			audio, err := autoTTS.Generate(
				TTS.text,
				voice,
				voice_settings,
			)
			if err != nil {
				fmt.Println("Press Enter to exit...")
        		fmt.Scanln()
				return
			}

			name := strings.ReplaceAll(strings.ToLower(TTS.speaker), " ", "_")
			
			file_name := fmt.Sprintf("%s/%d_%s.mp3", tts_directory, index, name)
			err = ioutil.WriteFile(file_name, audio, 0644)
			if err != nil {
				fmt.Println("Press Enter to exit...")
        		fmt.Scanln()
				return
			}
			
			autoTTS.PlayAudio(file_name)

			audio_prompt := fmt.Sprintf(
				"[?] Keep audio? [%s] %s | [%s] %s> ",
				bold_green.Sprint("Y"),
				bold_green.Sprint("yes"),
				bold_red.Sprint("N"),
				bold_red.Sprint("no"),
			)

			resp = ReceiveInput(audio_prompt)
			resp = strings.ToLower(resp)
			if resp != "n" {break}

			fmt.Println()
		}
	}
}