import { readFileSync } from 'node:fs';
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it } from 'vitest';
import { setupServer } from 'msw/node';
import { http, HttpResponse, delay } from 'msw';
import { fireEvent, screen, waitFor } from '@testing-library/dom';
import { mount } from './main';
const fixture = readFileSync('index.html', 'utf8');
const snapshot = { metadata: { title: 'Título original', author: 'Autora', date: '', tags: ['Artigos'], language: 'pt-BR', site: 'Exemplo', description: '' }, source: 'https://example.org/article', excerpt: 'Um artigo legível.', warnings: [] };
const server = setupServer();
let dispose: () => void;
beforeAll(() => server.listen({ onUnhandledFrame: 'error' }));
afterAll(() => server.close());
beforeEach(() => { document.body.innerHTML = fixture; localStorage.clear(); server.use(http.get('/api/v1/session', () => HttpResponse.json({ csrf_token: 'test-csrf' })), http.post('/api/v1/previews', () => HttpResponse.json({ id: 'preview-1', state: 'ready', snapshot }))); });
afterEach(() => { dispose?.(); server.resetHandlers(); });
const inputURL = (value: string) => { const input = screen.getByLabelText(/Endereço do artigo/); fireEvent.input(input, { target: { value } }); return input; };
const submit = () => fireEvent.submit(document.querySelector('form')!);
describe('article submission', () => {
 it('debounces preparation and does not show premature validation errors', async () => {
  let captures = 0; server.use(http.post('/api/v1/previews', () => { captures++; return HttpResponse.json({ id: 'preview-1', state: 'ready', snapshot }); })); dispose = mount(document.querySelector('#app')!);
  inputURL('https://'); expect((screen.getByText('Informe um endereço HTTP(S) completo.') as HTMLElement).hidden).toBe(true);
  inputURL('https://example.org/one'); inputURL('https://example.org/two');
  await screen.findByDisplayValue('Título original'); expect(captures).toBe(1);
 });
 it('reuses preparation on submit and sends edited metadata once', async () => {
  let captures = 0, imports = 0; let sentTitle = '';
  server.use(http.post('/api/v1/previews', async () => { captures++; await delay(100); return HttpResponse.json({ id: 'preview-1', state: 'ready', snapshot }); }), http.post('/api/v1/imports', async ({ request }) => { imports++; const body = await request.json() as { metadata: { title: string } }; sentTitle = body.metadata.title; return HttpResponse.json({ id: 'import-1', state: 'queued', stage: 'queued' }); }), http.get('/api/v1/imports/import-1', () => HttpResponse.json({ id: 'import-1', state: 'succeeded', stage: 'confirmed', result: { title: sentTitle, url: 'https://calibre.example.org/book/1', duplicate: false } })));
  dispose = mount(document.querySelector('#app')!); inputURL('https://example.org/article');
  await screen.findByDisplayValue('Título original'); fireEvent.input(screen.getByLabelText(/Título/), { target: { value: 'Minha revisão' } }); submit(); submit();
  await screen.findByRole('link', { name: 'Abrir no Calibre' }); expect(captures).toBe(1); expect(imports).toBe(1); expect(sentTitle).toBe('Minha revisão');
 });
 it('waits for the same preparation when clicked before preview is ready', async () => {
  let captures = 0;
  server.use(http.post('/api/v1/previews', async () => { captures++; await delay(100); return HttpResponse.json({ id: 'preview-1', state: 'ready', snapshot }); }), http.post('/api/v1/imports', () => HttpResponse.json({ id: 'import-1', state: 'queued', stage: 'queued' })), http.get('/api/v1/imports/import-1', () => HttpResponse.json({ id: 'import-1', state: 'succeeded', stage: 'confirmed', result: { title: 'Título original', url: 'https://calibre.example.org/book/1', duplicate: false } })));
  dispose = mount(document.querySelector('#app')!); inputURL('https://example.org/article'); submit();
  await screen.findByRole('link', { name: 'Abrir no Calibre' }); expect(captures).toBe(1);
 });
 it('ignores a stale response after the URL changes', async () => {
  server.use(http.post('/api/v1/previews', async ({ request }) => { const { url } = await request.json() as { url: string }; await delay(url.endsWith('/one') ? 900 : 5); return HttpResponse.json({ id: url, state: 'ready', snapshot: { ...snapshot, metadata: { ...snapshot.metadata, title: url.endsWith('/one') ? 'Antigo' : 'Atual' } } }); }));
  dispose = mount(document.querySelector('#app')!); inputURL('https://example.org/one'); await new Promise(r => setTimeout(r, 650)); inputURL('https://example.org/two'); await screen.findByDisplayValue('Atual'); expect(screen.queryByDisplayValue('Antigo')).toBeNull();
 });
 it('restores an accepted operation after reloading', async () => {
  localStorage.setItem('ptc:last-import', 'import-1'); server.use(http.get('/api/v1/imports/import-1', () => HttpResponse.json({ id: 'import-1', state: 'succeeded', stage: 'confirmed', result: { title: 'Recuperado', url: 'https://calibre.example.org/book/1', duplicate: true } })));
  dispose = mount(document.querySelector('#app')!); await screen.findByRole('link', { name: 'Abrir no Calibre' }); expect(screen.getByRole('status').textContent).toContain('já está');
 });
 it('preserves edits after a recoverable import error', async () => {
  server.use(http.post('/api/v1/imports', () => HttpResponse.json({ error: { code: 'unavailable', message: 'Calibre indisponível.', stage: 'enqueue' } }, { status: 503 })));
  dispose = mount(document.querySelector('#app')!); inputURL('https://example.org/article'); await screen.findByDisplayValue('Título original'); fireEvent.input(screen.getByLabelText(/Título/), { target: { value: 'Não perder' } }); submit(); await waitFor(() => expect(screen.getByRole('alert').textContent).toContain('indisponível')); expect(screen.getByDisplayValue('Não perder')).toBeTruthy(); expect(screen.getByRole('button', { name: 'Enviar ao Calibre' })).toBeTruthy();
 });
});

it('renews an expired preview without discarding edits', async () => {
 let captures = 0;
 server.use(http.post('/api/v1/previews', () => { captures++; return HttpResponse.json({ id: `preview-${captures}`, state: 'ready', snapshot }); }), http.post('/api/v1/imports', () => HttpResponse.json({ error: { code: 'preview_expired', message: 'Prévia expirada.', stage: 'enqueue' } }, { status: 410 })));
 dispose = mount(document.querySelector('#app')!); inputURL('https://example.org/article'); await screen.findByDisplayValue('Título original');
 fireEvent.input(screen.getByLabelText(/Título/), { target: { value: 'Edição preservada' } }); submit();
 await waitFor(() => expect(screen.getByRole('alert').textContent).toContain('Prévia renovada'));
 expect(screen.getByDisplayValue('Edição preservada')).toBeTruthy(); expect(captures).toBe(2);
});
it('replays the same idempotency key when an accepted response was lost', async () => {
 const payload = { preview_id: 'preview-1', metadata: snapshot.metadata };
 localStorage.setItem('ptc:pending', JSON.stringify({ payload, key: 'persistent-key-123456' }));
 let key = '';
 server.use(http.post('/api/v1/imports', ({ request }) => { key = request.headers.get('Idempotency-Key') ?? ''; return HttpResponse.json({ id: 'recovered-1', state: 'queued', stage: 'queued' }); }), http.get('/api/v1/imports/recovered-1', () => HttpResponse.json({ id: 'recovered-1', state: 'succeeded', stage: 'confirmed', result: { title: 'Recuperado', url: 'https://calibre.example.org/#book_id=1', duplicate: false } })));
 dispose = mount(document.querySelector('#app')!); await screen.findByRole('link', { name: 'Abrir no Calibre' }); expect(key).toBe('persistent-key-123456'); expect(localStorage.getItem('ptc:pending')).toBeNull();
});
