# ChatGPT-API-Proxy
![GitHub](https://img.shields.io/github/license/Heng-Bian/ChatGPT-API-Proxy)
![GitHub](https://img.shields.io/badge/build-pass-green)  
A reverse proxy of https://api.openai.com that supports token load-balance and avoids token leakage

According to openai, `Do not share your API key with others, or expose it in the browser or other client-side code.`
Reverse proxy of openai keeps your token secret by offering another authentication.

openai api reference
`https://platform.openai.com/docs/api-reference`
## Feature

- simple, clean but efficent code
- providing an authentication without openai token leakage
- supproting token load-balance
- avoiding the limitation of single openai token
- removing invalid token automatically

## Quick start
For help
```
./ChatGPT-API-Proxy -help
```
Start reverse proxy on port 8080 with your openai tokens
```
./ChatGPT-API-Proxy -prot 8080 -auth YOUR_AUTHORIZATION -tokens YOUR_OPENAI_TOKEN_1,YOUR_OPENAI_TOKEN_2
```
Start reverse proxy with config file
```
./ChatGPT-API-Proxy -config config.json
```
Use by cURL
```
curl --location 'http://localhost:8080/v1/chat/completions' \
--header 'Authorization: YOUR_AUTHORIZATION' \
--header 'Content-Type: application/json' \
--data '{
    "max_tokens": 250,
    "model": "gpt-3.5-turbo",
    "messages": [
        {
            "role": "user",
            "content": "Hello!"
        }
    ]
}'
```
