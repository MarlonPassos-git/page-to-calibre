import { test, expect } from '@playwright/test';
import { randomUUID } from 'node:crypto';
import { mkdir } from 'node:fs/promises';
test('preview, edit, import, reload and deduplicate against real Calibre', async ({ page }, info) => {
 const url = `https://example.org/synthetic-${randomUUID()}`;
 await page.goto('/');
 await page.getByLabel(/Endereço do artigo/).fill(url);
 await expect(page.getByLabel(/^Título/)).toHaveValue('Leitura de teste');
 await page.getByLabel(/^Título/).fill('Minha leitura revisada');
 await page.getByLabel(/Etiquetas/).fill('Artigos, Teste integrado');
 if (process.env.PTC_CAPTURE_REVIEW === '1') {
  await mkdir('../.impeccable/review', { recursive: true });
  await page.screenshot({ path: `../.impeccable/review/${info.project.name}.png`, fullPage: true });
 }
 await page.getByRole('button', { name: 'Enviar ao Calibre' }).click();
 await expect(page.getByRole('link', { name: 'Abrir no Calibre' })).toBeVisible({ timeout: 30000 });
 const link = await page.getByRole('link', { name: 'Abrir no Calibre' }).getAttribute('href');
 await page.reload();
 await expect(page.getByRole('link', { name: 'Abrir no Calibre' })).toHaveAttribute('href', link!);
 await page.getByLabel(/Endereço do artigo/).fill(url);
 await expect(page.getByLabel(/^Título/)).toHaveValue('Leitura de teste');
 await page.getByRole('button', { name: 'Enviar ao Calibre' }).click();
 await expect(page.getByRole('status')).toContainText('já está na biblioteca', { timeout: 30000 });
 await expect(page.getByRole('link', { name: 'Abrir no Calibre' })).toHaveAttribute('href', link!);
 await expect(page.getByText('Minha leitura revisada', { exact: false })).toBeVisible();
});
test('keyboard focus and viewport have no horizontal overflow', async ({ page }) => {
 await page.goto('/'); await page.keyboard.press('Tab'); await expect(page.getByLabel(/Endereço do artigo/)).toBeFocused();
 await page.keyboard.press('Tab'); await expect(page.getByRole('button', { name: 'Enviar ao Calibre' })).toBeFocused();
 expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});
