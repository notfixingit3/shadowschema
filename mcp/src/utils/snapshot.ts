export interface SpecDiffResult {
  session_key: string;
  previous_count: number;
  current_count: number;
  new_paths: string[];
  removed_paths: string[];
  unchanged: boolean;
}

export class SpecSnapshotStore {
  private snapshots = new Map<string, Set<string>>();

  private key(sessionId?: number): string {
    return sessionId === undefined ? "active" : String(sessionId);
  }

  diff(sessionId: number | undefined, currentPaths: string[]): SpecDiffResult {
    const sessionKey = this.key(sessionId);
    const previous = this.snapshots.get(sessionKey) ?? new Set<string>();
    const current = new Set(currentPaths);

    const newPaths: string[] = [];
    const removedPaths: string[] = [];

    for (const path of current) {
      if (!previous.has(path)) {
        newPaths.push(path);
      }
    }
    for (const path of previous) {
      if (!current.has(path)) {
        removedPaths.push(path);
      }
    }

    newPaths.sort();
    removedPaths.sort();
    this.snapshots.set(sessionKey, current);

    return {
      session_key: sessionKey,
      previous_count: previous.size,
      current_count: current.size,
      new_paths: newPaths,
      removed_paths: removedPaths,
      unchanged: newPaths.length === 0 && removedPaths.length === 0,
    };
  }

  reset(sessionId?: number): void {
    this.snapshots.delete(this.key(sessionId));
  }
}