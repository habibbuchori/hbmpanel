"use client";
import { useState } from "react";
import useSWR, { mutate } from "swr";
import { Button, Card, Input } from "@/components/ui";
import { api, fetcher } from "@/lib/api";

export default function DatabasePage() {
  const { data: dbs } = useSWR<string[]>("/api/postgres/databases", fetcher);
  const { data: users } = useSWR<string[]>("/api/postgres/users", fetcher);

  const [dbName, setDBName] = useState("");
  const [dbOwner, setDBOwner] = useState("");
  const [userName, setUserName] = useState("");
  const [userPass, setUserPass] = useState("");

  async function createDB(e: React.FormEvent) {
    e.preventDefault();
    await api("/api/postgres/databases", {
      method: "POST",
      body: JSON.stringify({ name: dbName, owner: dbOwner }),
    });
    setDBName(""); setDBOwner("");
    mutate("/api/postgres/databases");
  }
  async function createUser(e: React.FormEvent) {
    e.preventDefault();
    await api("/api/postgres/users", {
      method: "POST",
      body: JSON.stringify({ name: userName, password: userPass }),
    });
    setUserName(""); setUserPass("");
    mutate("/api/postgres/users");
  }

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold">PostgreSQL</h1>

      <div className="grid md:grid-cols-2 gap-4">
        <Card>
          <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">Databases</h2>
          <form onSubmit={createDB} className="space-y-2 mb-3">
            <Input placeholder="db name" value={dbName} onChange={(e) => setDBName(e.target.value)} />
            <Input placeholder="owner role" value={dbOwner} onChange={(e) => setDBOwner(e.target.value)} />
            <Button type="submit" className="w-full">Create database</Button>
          </form>
          <ul className="text-sm font-mono space-y-1">
            {(dbs ?? []).map((d) => <li key={d}>· {d}</li>)}
          </ul>
        </Card>

        <Card>
          <h2 className="text-sm font-semibold uppercase tracking-wider text-muted mb-3">Roles</h2>
          <form onSubmit={createUser} className="space-y-2 mb-3">
            <Input placeholder="role name" value={userName} onChange={(e) => setUserName(e.target.value)} />
            <Input placeholder="password" type="password" value={userPass} onChange={(e) => setUserPass(e.target.value)} />
            <Button type="submit" className="w-full">Create role</Button>
          </form>
          <ul className="text-sm font-mono space-y-1">
            {(users ?? []).map((u) => <li key={u}>· {u}</li>)}
          </ul>
        </Card>
      </div>
    </div>
  );
}
