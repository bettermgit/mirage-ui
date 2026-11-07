import { fireEvent, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AllSetting } from '@/models/setting';
import { renderWithProviders } from './test-utils';

const state = {
  sources: [
    {
      id: 's1',
      name: 'Airport A',
      url: 'https://a.example/sub',
      enable: true,
      lastFetchAt: 1730000000,
      lastCount: 12,
      lastError: '',
    },
  ],
  publications: [
    {
      id: 'p1',
      token: 'TOKEN123',
      name: 'Main line',
      sourceIds: ['s1'],
      subIds: [],
      enable: true,
      createdAt: 1730000000,
    },
  ],
};

const mutations = {
  saveSource: vi.fn().mockResolvedValue({ success: true }),
  deleteSource: vi.fn(),
  refreshSource: vi.fn(),
  savePublication: vi.fn().mockResolvedValue({ success: true }),
  deletePublication: vi.fn(),
  preview: vi.fn(),
  pending: false,
};

vi.mock('@/api/queries/useSubConvWorkbench', () => ({
  useSubConvStateQuery: () => ({
    sources: state.sources,
    publications: state.publications,
    loading: false,
    fetched: true,
    fetchError: '',
    refetch: vi.fn(),
  }),
  useWorkbenchClientsQuery: () => ({ data: [{ email: 'a@x', subId: 'sub-a' }] }),
  useSubConvMutations: () => mutations,
}));

import MihomoEngineTab from '@/pages/subscription/MihomoEngineTab';

describe('Mihomo engine workbench tab', () => {
  beforeEach(() => vi.clearAllMocks());

  function renderTab() {
    return renderWithProviders(
      <MemoryRouter>
        <MihomoEngineTab allSetting={new AllSetting()} updateSetting={vi.fn()} />
      </MemoryRouter>,
    );
  }

  it('renders sources with fetch status and publications with private links', () => {
    renderTab();
    expect(screen.getByText('Airport A')).toBeTruthy();
    expect(screen.getByText('12 nodes')).toBeTruthy();
    expect(screen.getByText('Main line')).toBeTruthy();
    expect(screen.getByText(/\/subconv\/TOKEN123/)).toBeTruthy();
  });

  it('toggling a source disables it through the save mutation', async () => {
    renderTab();
    const [sourceSwitch] = screen.getAllByRole('switch');
    fireEvent.click(sourceSwitch);
    await waitFor(() => expect(mutations.saveSource).toHaveBeenCalled());
    const payload = mutations.saveSource.mock.calls[0][0];
    expect(payload).toMatchObject({ id: 's1', enable: false });
  });

  it('keeps preview disabled until a source or client is selected', () => {
    renderTab();
    expect(screen.getByRole('button', { name: /Render preview/i }).hasAttribute('disabled')).toBe(
      true,
    );
  });
});
