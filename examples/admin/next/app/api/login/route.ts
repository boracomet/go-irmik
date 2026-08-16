import { cookies } from "next/headers";

export async function POST(request: Request) {
  const response = await fetch(`${process.env.IRMIK_URL}/api/v1/token`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: await request.text(),
  });
  if (!response.ok) return new Response("unauthorized", { status: 401 });
  const token = await response.json();
  (await cookies()).set("irmik_access", token.access_token, {
    httpOnly: true, sameSite: "lax", secure: process.env.NODE_ENV === "production",
  });
  return Response.json({ expires_at: token.expires_at });
}
