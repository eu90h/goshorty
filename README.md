This is an example Go program presenting an incredibly simple API for shortening urls. Posting "url=https://www.google.com" to the `/` endpoint will create a short name for "https://www.google.com" that can be looked up by GETting `/:shortname`.

E.g. to shorten a link,

```
curl -X POST https://goshorty.fly.dev/shorten -d url="https://www.google.com/"
{"short_url":"EBhU0623h5UWOUx","true_url":"https://www.google.com/"}

```

When you visit the shortened link, a redirection occurs:

```
curl -X GET https://goshorty.fly.dev/EBhU0623h5UWOUx
<a href="https://www.google.com/">Moved Permanently</a>.
```

An example of the app is running at https://goshorty.fly.dev/