import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { z } from 'zod';

import { HttpUtil } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { keys } from '@/api/queryKeys';
import { ClientRecordSchema } from '@/schemas/client';
import { SubConvStateSchema, type SubConvSource } from '@/generated/zod';

export type { SubConvSource, SubConvPublication } from '@/generated/zod';

type Msg = { success?: boolean; msg?: string; obj?: unknown };

async function fetchState() {
  const msg = await HttpUtil.get('/panel/api/subconv/state', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load engine workbench state');
  return parseMsg(msg, SubConvStateSchema, 'subconv/state').obj;
}

export function useSubConvStateQuery() {
  const query = useQuery({
    queryKey: keys.subConv.state(),
    queryFn: fetchState,
  });
  const state = query.data ?? { sources: [], publications: [] };
  return {
    sources: state.sources,
    publications: state.publications,
    loading: query.isFetching,
    fetched: query.data !== undefined || query.isError,
    fetchError: query.error ? (query.error as Error).message : '',
    refetch: query.refetch,
  };
}

// Subconv handlers bind nested structs from JSON only (no form tags); the
// form default of HttpUtil.post would flatten them into empty models.
const JSON_HEADERS = { 'Content-Type': 'application/json' };

export async function postSubConv(url: string, payload: unknown): Promise<Msg> {
  return (await HttpUtil.post(url, payload, { headers: JSON_HEADERS })) as Msg;
}

export function useSubConvMutations() {
  const queryClient = useQueryClient();

  const run = useMutation({
    mutationFn: ({ url, payload }: { url: string; payload: unknown }) => postSubConv(url, payload),
    onSuccess: (msg) => {
      if (msg?.success) queryClient.invalidateQueries({ queryKey: keys.subConv.root() });
    },
  });

  const call = (url: string, payload?: unknown) => run.mutateAsync({ url, payload });

  return {
    saveSource: (source: Partial<SubConvSource>) => call('/panel/api/subconv/source/save', source),
    deleteSource: (id: string) => call('/panel/api/subconv/source/delete', { id }),
    refreshSource: (id: string) => call('/panel/api/subconv/source/refresh', { id }),
    savePublication: (publication: unknown, rotateToken = false) =>
      call('/panel/api/subconv/publication/save', { publication, rotateToken }),
    deletePublication: (id: string) => call('/panel/api/subconv/publication/delete', { id }),
    preview: (sourceIds: string[], subIds: string[]) =>
      postSubConv('/panel/api/subconv/preview', { sourceIds, subIds }),
    pending: run.isPending,
  };
}

export interface WorkbenchClient {
  email: string;
  subId: string;
}

async function fetchClientsWithSubId(): Promise<WorkbenchClient[]> {
  const msg = await HttpUtil.get('/panel/api/clients/list', undefined, { silent: true });
  if (!msg?.success) throw new Error(msg?.msg || 'Failed to load clients');
  const list = parseMsg(msg, z.array(ClientRecordSchema).nullable(), 'clients/list').obj ?? [];
  return list
    .filter((c) => (c.subId ?? '').trim() !== '')
    .map((c) => ({ email: c.email, subId: c.subId as string }))
    .sort((a, b) => a.email.localeCompare(b.email));
}

export function useWorkbenchClientsQuery(enabled = true) {
  return useQuery({
    queryKey: keys.clients.all(),
    queryFn: fetchClientsWithSubId,
    enabled,
    staleTime: 30_000,
  });
}
