# Next.js BFF example

This App Router example keeps Irmik tokens on the Next server in an `httpOnly`
cookie. Browser components call `/api/items`; they never receive a Bearer token
or the Irmik URL. Do not put access tokens in `localStorage`.

Set `IRMIK_URL` to the admin API URL, then run this app with Next.js. CORS is
not needed for BFF server-to-server requests; browser-to-Irmik requests require
an explicit Irmik CORS origin.
