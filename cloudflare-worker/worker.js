/**
 * HBMPanel License Validation API
 * Runs on Cloudflare Workers with KV storage
 */

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    // Health check
    if (url.pathname === "/_health" && request.method === "GET") {
      return jsonResponse({ status: "ok" }, 200);
    }

    // Validation endpoint
    if (url.pathname === "/validate" && request.method === "POST") {
      return handleValidate(request, env);
    }

    // Admin endpoint
    if (url.pathname === "/admin/tokens" && request.method === "POST") {
      return handleAdminTokens(request, env);
    }

    return jsonResponse({ error: "not found" }, 404);
  },
};

async function handleValidate(request, env) {
  let body;
  try {
    body = await request.json();
  } catch {
    return jsonResponse({ valid: false, message: "invalid request body" }, 400);
  }

  const { token, machine_id, version } = body;
  if (!token || !machine_id) {
    return jsonResponse({ valid: false, message: "token and machine_id required" }, 400);
  }

  // Lookup token in KV
  const raw = await env.LICENSES.get(token);
  if (!raw) {
    return jsonResponse({ valid: false, message: "invalid or unknown token" });
  }

  const lic = JSON.parse(raw);

  // Check if active
  if (!lic.active) {
    return jsonResponse({ valid: false, message: "license deactivated" });
  }

  // Check expiry
  if (lic.expires_at && new Date(lic.expires_at) < new Date()) {
    return jsonResponse({
      valid: false,
      message: "license expired",
      plan: lic.plan,
    });
  }

  // Machine ID binding: first time, lock to this machine
  if (!lic.machine_id) {
    lic.machine_id = machine_id;
    await env.LICENSES.put(token, JSON.stringify(lic));
  } else if (lic.machine_id !== machine_id) {
    return jsonResponse({
      valid: false,
      message: "license bound to different server",
    });
  }

  return jsonResponse({
    valid: true,
    plan: lic.plan,
    features: lic.features || [],
    expires_at: lic.expires_at || "",
    message: "ok",
  });
}

async function handleAdminTokens(request, env) {
  // Verify admin key
  const adminKey = request.headers.get("X-Admin-Key");
  if (adminKey !== env.ADMIN_KEY) {
    return jsonResponse({ error: "forbidden" }, 403);
  }

  let body;
  try {
    body = await request.json();
  } catch {
    return jsonResponse({ error: "invalid request body" }, 400);
  }

  const { token, plan, features, expires_at } = body;
  if (!token || !plan) {
    return jsonResponse({ error: "token and plan required" }, 400);
  }

  const licenseData = {
    plan,
    features: features || [],
    expires_at: expires_at || null,
    active: true,
    machine_id: null,
  };

  await env.LICENSES.put(token, JSON.stringify(licenseData));
  return jsonResponse({ ok: true, message: "token added" });
}

function jsonResponse(data, status = 200) {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
