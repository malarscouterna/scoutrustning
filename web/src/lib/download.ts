export function toCsv(rows: string[][]): string {
	const escape = (value: string) => {
		const text = value ?? '';
		return /[",\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text;
	};
	const protect = (value: string) => {
		const text = value ?? '';
		// Prevent spreadsheet formula injection from user-controlled issue text.
		// Covers ASCII and full-width formula prefixes plus tab/newline starts.
		if (/^[=+\-@\t\r\n＝＋－＠]/.test(text)) {
			return `'${text}`;
		}
		return text;
	};
	// BOM keeps Swedish characters readable when the file is opened in Excel.
	return `\uFEFF${rows
		.map((row) => row.map((cell) => escape(protect(cell))).join(','))
		.join('\r\n')}\r\n`;
}

export function downloadTextFile(filename: string, text: string, mimeType: string): void {
	if (typeof document === 'undefined') return;

	const blob = new Blob([text], { type: mimeType });
	const url = URL.createObjectURL(blob);
	const link = document.createElement('a');
	link.href = url;
	link.download = filename;
	document.body.appendChild(link);
	link.click();
	link.remove();
	setTimeout(() => URL.revokeObjectURL(url), 0);
}

export function sanitizeFileName(value: string): string {
	const name = value
		.replace(/[\\/:*?"<>|]/g, '-')
		.replace(/\s+/g, ' ')
		.trim()
		.replace(/[. ]+$/g, '')
		.slice(0, 120)
		.trim();

	return name || 'export';
}
