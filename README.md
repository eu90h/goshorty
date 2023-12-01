This is an example Go program presenting an incredibly simple API for shortening urls. Posting "url=https://www.google.com" to the `/` endpoint will create a short name for "https://www.google.com" that can be looked up by GETting `/:shortname`.

E.g. to shorten a link,

```
curl -X POST 127.0.0.1:8080/ -d url=https://www.reddit.com

{"short_url":"OaJcohOGElMrdDL","true_url":"https://www.reddit.com"}
```
lookup shortened url

```
curl -X GET 127.0.0.1:8080/OaJcohOGElMrdDL

{"url":"https://www.reddit.com"}
```
