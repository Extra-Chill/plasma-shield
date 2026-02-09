/**
 * OpenClaw Plugin for Plasma Shield Integration
 * 
 * Intercepts exec tool calls and validates them against the Plasma Shield
 * security router before execution.
 */

export const name = "plasma-shield";
export const version = "0.1.0";

// Types for OpenClaw plugin system
interface ToolCallEvent {
  toolName: string;
  parameters: Record<string, unknown>;
  sessionId?: string;
  agentId?: string;
}

interface BlockResult {
  block?: boolean;
  blockReason?: string;
}

// Types for Plasma Shield API
interface ShieldCheckRequest {
  command: string;
  workdir?: string;
  env?: Record<string, string>;
  sessionId?: string;
  agentId?: string;
}

interface ShieldCheckResponse {
  allowed: boolean;
  reason?: string;
  riskLevel?: "low" | "medium" | "high" | "critical";
}

// Configuration from environment
const SHIELD_URL = process.env.PLASMA_SHIELD_URL || "http://localhost:3100";
const SHIELD_TOKEN = process.env.PLASMA_SHIELD_TOKEN || "";
const REQUEST_TIMEOUT_MS = 5000;

/**
 * Call the Plasma Shield API to check if a command is allowed
 */
async function checkWithShield(request: ShieldCheckRequest): Promise<ShieldCheckResponse> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);

  try {
    const response = await fetch(`${SHIELD_URL}/exec/check`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(SHIELD_TOKEN && { "Authorization": `Bearer ${SHIELD_TOKEN}` }),
      },
      body: JSON.stringify(request),
      signal: controller.signal,
    });

    if (!response.ok) {
      throw new Error(`Shield returned ${response.status}: ${response.statusText}`);
    }

    return await response.json() as ShieldCheckResponse;
  } finally {
    clearTimeout(timeoutId);
  }
}

/**
 * OpenClaw beforeToolCall hook
 * 
 * Intercepts exec calls and validates them with Plasma Shield.
 * Fails closed - if shield is unreachable, the command is blocked.
 */
export const beforeToolCall = async (event: ToolCallEvent): Promise<BlockResult> => {
  // Only intercept exec tool calls
  if (event.toolName !== "exec") {
    return {};
  }

  const command = event.parameters.command as string;
  if (!command) {
    return {};
  }

  const checkRequest: ShieldCheckRequest = {
    command,
    workdir: event.parameters.workdir as string | undefined,
    env: event.parameters.env as Record<string, string> | undefined,
    sessionId: event.sessionId,
    agentId: event.agentId,
  };

  try {
    const result = await checkWithShield(checkRequest);

    if (!result.allowed) {
      const reason = result.reason || "Command blocked by Plasma Shield";
      console.log(`[plasma-shield] BLOCKED: "${command}" - ${reason} (risk: ${result.riskLevel || "unknown"})`);
      return {
        block: true,
        blockReason: reason,
      };
    }

    // Command allowed
    return {};

  } catch (error) {
    // Fail closed: if we can't reach the shield, block the command
    const errorMessage = error instanceof Error ? error.message : String(error);
    const isTimeout = errorMessage.includes("abort");
    
    const reason = isTimeout
      ? "Plasma Shield timeout - command blocked (fail-closed)"
      : `Plasma Shield unreachable - command blocked (fail-closed): ${errorMessage}`;

    console.log(`[plasma-shield] BLOCKED (fail-closed): "${command}" - ${reason}`);
    
    return {
      block: true,
      blockReason: reason,
    };
  }
};
