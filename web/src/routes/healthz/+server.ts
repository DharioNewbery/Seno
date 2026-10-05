// Healthcheck do container web (usado pelo compose).
export async function GET() {
  return new Response(JSON.stringify({ status: "ok" }), {
    headers: { "content-type": "application/json" },
  });
}
