"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import {
  addGroupMember,
  createRecipient,
  createRecipientGroup,
  deleteRecipient,
  deleteRecipientGroup,
  listGroupMembers,
  listRecipientGroups,
  listRecipients,
  removeGroupMember,
  type Recipient,
  type RecipientGroup,
  type RecipientInput,
} from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { RecipientForm } from "@/app/ui/recipient-form";
import { RecipientGroupForm } from "@/app/ui/recipient-group-form";

const removeButtonClass =
  "rounded border border-black/[.1] px-2 py-1 text-xs transition-colors hover:bg-black/[.04] dark:border-white/[.15] dark:hover:bg-[#1a1a1a]";

// GroupMembers manages one group's membership: the recipients already in it,
// and a picker to add any of the account's other recipients.
function GroupMembers({
  token,
  group,
  recipients,
}: {
  token: string;
  group: RecipientGroup;
  recipients: Recipient[];
}) {
  const [members, setMembers] = useState<Recipient[] | null>(null);
  const [selected, setSelected] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    listGroupMembers(token, group.id)
      .then(setMembers)
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to load members"));
  }, [token, group.id]);

  async function handleAdd() {
    if (!selected) return;
    setError(null);
    try {
      await addGroupMember(token, group.id, selected);
      setMembers(await listGroupMembers(token, group.id));
      setSelected("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to add member");
    }
  }

  async function handleRemove(recipientId: string) {
    setError(null);
    try {
      await removeGroupMember(token, group.id, recipientId);
      setMembers(await listGroupMembers(token, group.id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to remove member");
    }
  }

  if (members === null) {
    return <p className="text-sm text-zinc-600 dark:text-zinc-400">Loading members…</p>;
  }

  const availableRecipients = recipients.filter(
    (recipient) => !members.some((member) => member.id === recipient.id),
  );

  return (
    <div className="flex flex-col gap-2 border-t border-black/[.1] pt-3 dark:border-white/[.15]">
      {members.length === 0 && (
        <p className="text-sm text-zinc-600 dark:text-zinc-400">No members yet.</p>
      )}
      {members.map((member) => (
        <div key={member.id} className="flex items-center justify-between text-sm">
          <span>
            {member.name || member.email} <span className="text-zinc-500">{member.email}</span>
          </span>
          <button onClick={() => handleRemove(member.id)} className="text-red-600 hover:underline">
            Remove
          </button>
        </div>
      ))}
      {availableRecipients.length > 0 && (
        <div className="flex items-center gap-2 pt-1">
          <select
            value={selected}
            onChange={(e) => setSelected(e.target.value)}
            className="rounded border border-black/[.1] px-2 py-1 text-sm dark:border-white/[.15] dark:bg-black"
          >
            <option value="">Add a recipient…</option>
            {availableRecipients.map((recipient) => (
              <option key={recipient.id} value={recipient.id}>
                {recipient.name || recipient.email}
              </option>
            ))}
          </select>
          <button onClick={handleAdd} disabled={!selected} className={removeButtonClass}>
            Add
          </button>
        </div>
      )}
      {error && <p className="text-sm text-red-600">{error}</p>}
    </div>
  );
}

export default function RecipientsPage() {
  const router = useRouter();
  const { token, logout } = useAuth();
  const [recipients, setRecipients] = useState<Recipient[] | null>(null);
  const [groups, setGroups] = useState<RecipientGroup[] | null>(null);
  const [expandedGroupId, setExpandedGroupId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    Promise.all([listRecipients(token), listRecipientGroups(token)])
      .then(([loadedRecipients, loadedGroups]) => {
        setRecipients(loadedRecipients);
        setGroups(loadedGroups);
      })
      .catch(() => {
        logout();
        router.replace("/login");
      });
  }, [token, router, logout]);

  async function handleCreateRecipient(input: RecipientInput) {
    if (!token) return;
    await createRecipient(token, input);
    setRecipients(await listRecipients(token));
  }

  async function handleDeleteRecipient(id: string) {
    if (!token) return;
    setError(null);
    try {
      await deleteRecipient(token, id);
      setRecipients(await listRecipients(token));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete recipient");
    }
  }

  async function handleCreateGroup(name: string) {
    if (!token) return;
    await createRecipientGroup(token, name);
    setGroups(await listRecipientGroups(token));
  }

  async function handleDeleteGroup(id: string) {
    if (!token) return;
    setError(null);
    try {
      await deleteRecipientGroup(token, id);
      if (expandedGroupId === id) setExpandedGroupId(null);
      setGroups(await listRecipientGroups(token));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete recipient group");
    }
  }

  if (!token || !recipients || !groups) {
    return (
      <div className="flex flex-1 items-center justify-center py-32">
        <p>Loading…</p>
      </div>
    );
  }

  return (
    <div className="flex flex-1 flex-col items-center gap-10 px-6 py-16">
      <div className="flex w-full max-w-md items-center justify-between">
        <h1 className="text-2xl font-semibold">Recipients</h1>
      </div>

      {error && <p className="w-full max-w-md text-sm text-red-600">{error}</p>}

      <div className="flex w-full max-w-md flex-col gap-3">
        {recipients.length === 0 && (
          <p className="text-sm text-zinc-600 dark:text-zinc-400">No recipients yet.</p>
        )}
        {recipients.map((recipient) => (
          <div
            key={recipient.id}
            className="flex items-center justify-between rounded border border-black/[.1] px-4 py-3 dark:border-white/[.15]"
          >
            <div>
              <p className="font-medium">{recipient.name || recipient.email}</p>
              <p className="text-sm text-zinc-600 dark:text-zinc-400">{recipient.email}</p>
            </div>
            <button
              onClick={() => handleDeleteRecipient(recipient.id)}
              className="text-sm text-red-600 hover:underline"
            >
              Delete
            </button>
          </div>
        ))}
      </div>

      <div className="flex w-full max-w-md flex-col gap-4 border-t border-black/[.1] pt-8 dark:border-white/[.15]">
        <h2 className="text-lg font-semibold">Add a recipient</h2>
        <RecipientForm onCreate={handleCreateRecipient} />
      </div>

      <div className="flex w-full max-w-md flex-col gap-3 border-t border-black/[.1] pt-8 dark:border-white/[.15]">
        <h2 className="text-lg font-semibold">Groups</h2>
        {groups.length === 0 && (
          <p className="text-sm text-zinc-600 dark:text-zinc-400">No recipient groups yet.</p>
        )}
        {groups.map((group) => (
          <div
            key={group.id}
            className="flex flex-col gap-2 rounded border border-black/[.1] px-4 py-3 dark:border-white/[.15]"
          >
            <div className="flex items-center justify-between">
              <p className="font-medium">{group.name}</p>
              <div className="flex items-center gap-4">
                <button
                  onClick={() => setExpandedGroupId(expandedGroupId === group.id ? null : group.id)}
                  className="text-sm underline"
                >
                  {expandedGroupId === group.id ? "Hide members" : "Manage members"}
                </button>
                <button
                  onClick={() => handleDeleteGroup(group.id)}
                  className="text-sm text-red-600 hover:underline"
                >
                  Delete
                </button>
              </div>
            </div>
            {expandedGroupId === group.id && (
              <GroupMembers token={token} group={group} recipients={recipients} />
            )}
          </div>
        ))}
      </div>

      <div className="flex w-full max-w-md flex-col gap-4 border-t border-black/[.1] pt-8 dark:border-white/[.15]">
        <h2 className="text-lg font-semibold">Create a group</h2>
        <RecipientGroupForm onCreate={handleCreateGroup} />
      </div>
    </div>
  );
}
