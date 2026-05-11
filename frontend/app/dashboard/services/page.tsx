"use client";
import useSWR, { mutate } from "swr";
import { Button, Card } from "@/components/ui";
import { api, fetcher } from "@/lib/api";

type Service = { name: string; active: string; sub: string };

export default function ServicesPage() {
  const { data } = useSWR<Service[]>("/api/system/services", fetcher, { refreshInterval: 5000 });

  async function act(name: string, action: string) {
    await api(`/api/system/services/${name}/${action}`, { method: "POST" });
    mutate("/api/system/services");
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">Services</h1>
      <Card>
        <div className="divide-y divide-line">
          {(data ?? []).map((s) => (
            <div key={s.name} className="flex items-center justify-between py-3">
              <div>
                <div className="font-mono">{s.name}</div>
                <div className="text-xs text-muted">{s.active} · {s.sub}</div>
              </div>
              <div className="flex gap-2">
                <Button variant="outline" onClick={() => act(s.name, "restart")}>Restart</Button>
                <Button variant="outline" onClick={() => act(s.name, "stop")}>Stop</Button>
                <Button onClick={() => act(s.name, "start")}>Start</Button>
              </div>
            </div>
          ))}
        </div>
      </Card>
    </div>
  );
}
