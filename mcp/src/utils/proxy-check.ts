import net from "node:net";

export async function isTcpPortOpen(host: string, port: number, timeoutMs = 2000): Promise<boolean> {
  return new Promise((resolve) => {
    const socket = new net.Socket();
    let settled = false;

    const finish = (value: boolean) => {
      if (settled) {
        return;
      }
      settled = true;
      socket.destroy();
      resolve(value);
    };

    socket.setTimeout(timeoutMs);
    socket.once("connect", () => finish(true));
    socket.once("timeout", () => finish(false));
    socket.once("error", () => finish(false));
    socket.connect(port, host);
  });
}

export function parseProxyUrl(proxyUrl: string): { host: string; port: number } {
  const url = new URL(proxyUrl);
  return {
    host: url.hostname,
    port: url.port ? Number.parseInt(url.port, 10) : url.protocol === "https:" ? 443 : 80,
  };
}