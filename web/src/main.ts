export interface Metadata { title: string; author: string; date: string; tags: string[]; language: string; site: string; description: string }
interface Snapshot { metadata: Metadata; source: string; excerpt: string; warnings: string[] }
interface Preview { id: string; state: string; snapshot?: Snapshot; error?: string }
interface Operation { id: string; state: string; stage: string; error?: string; warnings?: string[]; result?: { title: string; url: string; duplicate: boolean } }
class APIError extends Error { constructor(message: string, public code: string) { super(message); } }
const delay = (ms: number, signal?: AbortSignal) => new Promise<void>((resolve, reject) => {
 const abort = () => { clearTimeout(timer); reject(new DOMException('Aborted', 'AbortError')); };
 const timer = setTimeout(() => { signal?.removeEventListener('abort', abort); resolve(); }, ms);
 if (signal?.aborted) abort(); else signal?.addEventListener('abort', abort, { once: true });
});
function object(value: unknown): Record<string, unknown> { if (!value || typeof value !== 'object') throw new Error('Resposta inválida da API.'); return value as Record<string, unknown>; }
function previewValue(value: unknown): Preview { const p = object(value); if (typeof p.id !== 'string' || typeof p.state !== 'string') throw new Error('Prévia inválida.'); if (p.snapshot) { const s = object(p.snapshot); const m = object(s.metadata); for (const field of ['title', 'author', 'date', 'language', 'site', 'description']) if (typeof m[field] !== 'string') throw new Error('Metadados inválidos.'); if (!Array.isArray(m.tags) || !m.tags.every(t => typeof t === 'string') || typeof s.source !== 'string' || typeof s.excerpt !== 'string') throw new Error('Conteúdo inválido.'); } return p as unknown as Preview; }
function operationValue(value: unknown): Operation { const op = object(value); if (typeof op.id !== 'string' || typeof op.state !== 'string' || typeof op.stage !== 'string') throw new Error('Operação inválida.'); if (op.result) { const r = object(op.result); if (typeof r.title !== 'string' || typeof r.url !== 'string' || typeof r.duplicate !== 'boolean') throw new Error('Resultado inválido.'); } return op as unknown as Operation; }

export function mount(root: HTMLElement) {
 const find = <T extends HTMLElement>(id: string) => { const el = root.querySelector<T>(`#${id}`); if (!el) throw new Error(`Controle ausente: ${id}`); return el; };
 const form = find<HTMLFormElement>('send-form'), url = find<HTMLInputElement>('url'), send = find<HTMLButtonElement>('send');
 const fields = { title: find<HTMLInputElement>('title'), author: find<HTMLInputElement>('author'), date: find<HTMLInputElement>('date'), tags: find<HTMLInputElement>('tags') };
 const status = find('status'), error = find('error'), result = find('result'), section = find('preview');
 let version = 0, controller = new AbortController(), timer: ReturnType<typeof setTimeout> | undefined;
 let prepared: Preview | undefined, preparing: Promise<Preview> | undefined, busy = false, token = '', stopped = false;
 const listeners: (() => void)[] = [];
 function on<T extends HTMLElement>(el: T, event: string, fn: EventListener) { el.addEventListener(event, fn); listeners.push(() => el.removeEventListener(event, fn)); }
 function storageGet(key: string) { try { return localStorage.getItem(key); } catch { return null; } }
 function storageSet(key: string, value: string) { try { localStorage.setItem(key, value); } catch { /* API IDs still remain available in the result. */ } }
 function storageRemove(key: string) { try { localStorage.removeItem(key); } catch { /* Storage may be unavailable in private browsers. */ } }
 async function request(path: string, options: RequestInit = {}, refreshed = false): Promise<unknown> {
  if (options.method === 'POST' && !token) { const session = object(await request('/api/v1/session')); if (typeof session.csrf_token !== 'string') throw new Error('Sessão inválida.'); token = session.csrf_token; }
  const response = await fetch(path, { ...options, headers: { ...options.headers, ...(options.method === 'POST' ? { 'Content-Type': 'application/json', 'X-CSRF-Token': token } : {}) } });
  const body: unknown = await response.json();
  if (!response.ok) { const e = object(object(body).error); if (e.code === 'csrf_forbidden' && !refreshed) { token = ''; return request(path, options, true); } throw new APIError(typeof e.message === 'string' ? e.message : 'Falha na API.', typeof e.code === 'string' ? e.code : 'api_error'); }
  return body;
 }
 function showError(message: string) { error.textContent = message; error.hidden = false; status.textContent = ''; }
 function clearError() { error.hidden = true; error.textContent = ''; }
 function validURL() { try { const u = new URL(url.value.trim()); return ['http:', 'https:'].includes(u.protocol) && !u.username && !u.password; } catch { return false; } }
 function metadata(): Metadata { const base = prepared?.snapshot?.metadata; if (!base) throw new Error('Prepare a prévia primeiro.'); return { ...base, title: fields.title.value, author: fields.author.value, date: fields.date.value, tags: fields.tags.value.split(',').map(t => t.trim()).filter(Boolean) }; }
 function fill(p: Preview, keepEdits: boolean) {
  if (!p.snapshot) throw new Error('Prévia sem conteúdo.');
  const s = p.snapshot;
  if (!keepEdits) { fields.title.value = s.metadata.title; fields.author.value = s.metadata.author; fields.date.value = s.metadata.date; fields.tags.value = s.metadata.tags.join(', '); }
  for (const field of Object.values(fields)) field.disabled = false;
  find('source').textContent = `${s.metadata.site ? s.metadata.site + ' · ' : ''}${s.source}`;
  find('language').textContent = s.metadata.language ? `Idioma: ${s.metadata.language}` : 'Idioma não informado pela página.';
  find('excerpt').textContent = s.excerpt; find('warnings').textContent = s.warnings?.join(' ') ?? ''; section.hidden = false;
 }
 function prepare(keepEdits = false): Promise<Preview> {
  if (preparing) return preparing;
  const current = version, signal = controller.signal;
  status.textContent = 'Preparando a prévia…'; clearError();
  preparing = (async () => {
   let p = previewValue(await request('/api/v1/previews', { method: 'POST', body: JSON.stringify({ url: url.value.trim() }), signal }));
   while (p.state === 'queued') { await delay(400, signal); p = previewValue(await request(`/api/v1/previews/${p.id}`, { signal })); }
   if (current !== version) throw new DOMException('Aborted', 'AbortError');
   if (p.state !== 'ready') throw new Error(p.error ?? 'Não foi possível preparar o artigo.');
   prepared = p; fill(p, keepEdits); status.textContent = 'Prévia pronta. Você pode editar os detalhes.'; return p;
  })().finally(() => { if (current === version) preparing = undefined; });
  return preparing;
 }
 async function recover(id: string) {
  for (;;) {
   const op = operationValue(await request(`/api/v1/imports/${encodeURIComponent(id)}`)); if (stopped) return;
   if (op.state === 'succeeded' && op.result) {
    status.textContent = op.result.duplicate ? 'Este artigo já está na biblioteca.' : 'Artigo adicionado ao Calibre.';
    result.replaceChildren(document.createTextNode(`${op.result.title} · `));
    const link = document.createElement('a'); const target = new URL(op.result.url); if (!['http:', 'https:'].includes(target.protocol)) throw new Error('Link Calibre inválido.');
    link.href = target.href; link.textContent = 'Abrir no Calibre'; result.append(link); result.hidden = false;
    find('warnings').textContent = op.warnings?.join(' ') ?? ''; return;
   }
   if (op.state === 'failed' || op.state === 'uncertain') throw new Error(op.error ?? 'Envio não concluído.');
   status.textContent = op.stage === 'writing' ? 'Confirmando no Calibre…' : op.stage === 'packaging' ? 'Organizando o EPUB…' : 'Envio na fila…';
   await delay(600);
   if (stopped) return;
  }
 }
 async function accept(payload: { preview_id: string; metadata: Metadata }, key: string) {
  const op = operationValue(await request('/api/v1/imports', { method: 'POST', headers: { 'Idempotency-Key': key }, body: JSON.stringify(payload) }));
  storageSet('ptc:last-import', op.id); storageRemove('ptc:pending'); await recover(op.id);
 }
 function lock(value: boolean) { busy = value; send.disabled = value; url.readOnly = value; for (const field of Object.values(fields)) field.readOnly = value; send.textContent = value ? 'Enviando…' : 'Enviar ao Calibre'; }
 on(url, 'input', () => {
  if (busy) return; version++; controller.abort(); controller = new AbortController(); clearTimeout(timer); prepared = undefined; preparing = undefined; section.hidden = true; result.hidden = true; clearError(); status.textContent = ''; find('url-error').hidden = true; url.removeAttribute('aria-invalid');
  for (const field of Object.values(fields)) { field.value = ''; field.disabled = true; }
  if (validURL()) timer = setTimeout(() => { void prepare().catch(e => { if (e instanceof Error && e.name !== 'AbortError') showError(e.message); }); }, 600);
 });
 on(url, 'blur', () => { const invalid = url.value !== '' && !validURL(); find('url-error').hidden = !invalid; url.setAttribute('aria-invalid', String(invalid)); });
 on(form, 'submit', (event) => {
  event.preventDefault(); if (busy) return;
  if (!validURL()) { showError('Informe um endereço HTTP(S) completo.'); url.focus(); return; }
  if (!form.reportValidity()) return;
  clearTimeout(timer); clearError(); result.hidden = true; lock(true);
  void (async () => {
   if (!prepared) await prepare();
   if (!form.reportValidity()) return;
   const payload = { preview_id: prepared!.id, metadata: metadata() }, key = crypto.randomUUID();
   storageSet('ptc:pending', JSON.stringify({ payload, key }));
   try { await accept(payload, key); } catch (e) {
    if (e instanceof APIError && e.code === 'preview_expired') { storageRemove('ptc:pending'); prepared = undefined; await prepare(true); throw new Error('Prévia renovada. Suas edições foram mantidas; envie novamente.', { cause: e }); }
    throw e;
   }
  })().catch(e => { showError(e instanceof Error ? e.message : 'Falha no envio.'); }).finally(() => lock(false));
 });
 void (async () => {
  const pending = storageGet('ptc:pending'), id = storageGet('ptc:last-import'); if (!pending && !id) return;
  lock(true);
  try { if (pending) { const parsed = object(JSON.parse(pending) as unknown); if (typeof parsed.key !== 'string') throw new Error('Envio salvo inválido.'); const payload = object(parsed.payload); if (typeof payload.preview_id !== 'string') throw new Error('Envio salvo inválido.'); await accept(payload as unknown as { preview_id: string; metadata: Metadata }, parsed.key); } else if (id) await recover(id); }
  catch (e) { showError(e instanceof Error ? e.message : 'Não foi possível recuperar o envio.'); }
  finally { lock(false); }
 })();
 return () => { stopped = true; controller.abort(); clearTimeout(timer); listeners.forEach(remove => remove()); };
}
