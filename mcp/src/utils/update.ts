import { exec } from "child_process";
import { promisify } from "util";

const execAsync = promisify(exec);

export async function tryAutoUpdate(mcpDir: string, enabled: boolean): Promise<void> {
  if (!enabled) return;

  try {
    await execAsync("git rev-parse --is-inside-work-tree", { cwd: mcpDir });
  } catch {
    return; // Not a git repo or git not installed
  }

  // Run the update check
  try {
    // 1. Fetch remote updates with 5s timeout
    await execAsync("git fetch", { cwd: mcpDir, timeout: 5000 });

    // 2. Compare revisions
    const { stdout: localOut } = await execAsync("git rev-parse HEAD", { cwd: mcpDir });
    const { stdout: remoteOut } = await execAsync("git rev-parse @{u}", { cwd: mcpDir });
    const { stdout: baseOut } = await execAsync("git merge-base HEAD @{u}", { cwd: mcpDir });

    const local = localOut.trim();
    const remote = remoteOut.trim();
    const base = baseOut.trim();

    if (local === remote) {
      return; // Up to date
    }

    if (local === base) {
      console.error("[shadowschema-mcp] Auto-update: New changes found on remote. Pulling updates...");
      await execAsync("git pull", { cwd: mcpDir, timeout: 10000 });

      console.error("[shadowschema-mcp] Auto-update: Rebuilding TypeScript files...");
      await execAsync("npm run build", { cwd: mcpDir, timeout: 20000 });

      console.error(
        "[shadowschema-mcp] Auto-update: Successfully updated to latest version. Changes will be active on the next startup.",
      );
    }
  } catch (err) {
    console.error("[shadowschema-mcp] Auto-update failed or skipped:", String(err));
  }
}
