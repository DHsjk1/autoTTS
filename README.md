## About
AutoTTS is a program which allows you to easily generate tts using ElevenLabs API from a script.
![screenshot](https://github.com/DHsjk1/autoTTS/assets/128737623/074a9b92-7213-477e-9c96-e98a2dbf4c89)

## Requirements
- Color library by Faith: `github.com/fatih/color`
- An ElevenLabs API key

## Getting Started
- Download an executeable from the releases tab

OR
- Download the reposity `https://github.com/DHsjk1/autoTTS.git`
- Build the executeable `go build autoTTS.go`

## How to use
- Place a .txt inside the directory where the autoTTS executeable/script is
- On first launch you will be prompted to generate a config by typing in your API key
- After the config is generated, you should edit the voices list to include your speakers and their voices (either voice ID or Name)

Example script file:
```
Speaker1: Hi
Speaker2: Hello!
```

To get your API key go to `https://elevenlabs.io/` then `My Account > Profile + API key`
