import { cookies } from "next/headers";

export async function GET() {
  const token = (await cookies()).get("irmik_access")?.value;
  if (!token) return new Response("unauthorized", { status: 401 });
  const response = await fetch(`${process.env.IRMIK_URL}/api/v1/items`, {
    headers: { authorization: `Bearer ${token}` },
    cache: "no-store",
  });
  return new Response(await response.text(), {
    status: response.status, headers: { "content-type": "application/json" },
  });
}
