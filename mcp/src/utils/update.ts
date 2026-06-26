import { exec } from "child_process";
import { promisify } from "util";

const execAsync = promisify(exec);

async function getGitRoot(startDir: string): Promise<string | null> {
  try {
    const { stdout } = await execAsync("git rev-parse --show-toplevel", { cwd: startDir });
    return stdout.trim() || null;
  } catch {
    return null;
  }
}

async function resolveUpstreamRef(gitRoot: string): Promise<string | null> {
  try {
    const { stdout } = await execAsync("git rev-parse --abbrev-ref @{u}", { cwd: gitRoot });
    const upstream = stdout.trim();
    return upstream || null;
  } catch {
    // Branch has no tracking upstream — try origin/<current-branch>.
  }

  try {
    const { stdout: branchOut } = await execAsync("git rev-parse --abbrev-ref HEAD", {
      cwd: gitRoot,
    });
    const branch = branchOut.trim();
    if (!branch || branch === "HEAD") return null;

    const candidate = `origin/${branch}`;
    await execAsync(`git rev-parse --verify ${candidate}^{commit}`, { cwd: gitRoot });
    return candidate;
  } catch {
    return null;
  }
}

export async function tryAutoUpdate(mcpDir: string, enabled: boolean): Promise<void> {
  if (!enabled) return;

  const gitRoot = await getGitRoot(mcpDir);
  if (!gitRoot) return;

  try {
    await execAsync("git fetch", { cwd: gitRoot, timeout: 5000 });

    const upstream = await resolveUpstreamRef(gitRoot);
    if (!upstream) return;

    const { stdout: localOut } = await execAsync("git rev-parse HEAD", { cwd: gitRoot });
    const { stdout: remoteOut } = await execAsync(`git rev-parse ${upstream}`, { cwd: gitRoot });
    const { stdout: baseOut } = await execAsync(`git merge-base HEAD ${upstream}`, {
      cwd: gitRoot,
    });

    const local = localOut.trim();
    const remote = remoteOut.trim();
    const base = baseOut.trim();

    if (local === remote) {
      return;
    }

    if (local === base) {
      console.error("[shadowschema-mcp] Auto-update: New changes found on remote. Pulling updates...");
      await execAsync("git pull", { cwd: gitRoot, timeout: 10000 });

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