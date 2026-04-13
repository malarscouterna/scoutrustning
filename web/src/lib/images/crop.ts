import type { CropArea } from 'svelte-easy-crop';

export async function cropImage(imageSrc: string, pixelCrop: CropArea): Promise<Blob> {
	const image = await loadImage(imageSrc);
	const canvas = document.createElement('canvas');
	canvas.width = pixelCrop.width;
	canvas.height = pixelCrop.height;
	const ctx = canvas.getContext('2d')!;
	ctx.drawImage(
		image,
		pixelCrop.x, pixelCrop.y, pixelCrop.width, pixelCrop.height,
		0, 0, pixelCrop.width, pixelCrop.height
	);
	return new Promise((resolve, reject) => {
		canvas.toBlob(
			(blob) => blob ? resolve(blob) : reject(new Error('Canvas toBlob failed')),
			'image/jpeg',
			0.92
		);
	});
}

function loadImage(src: string): Promise<HTMLImageElement> {
	return new Promise((resolve, reject) => {
		const img = new Image();
		img.onload = () => resolve(img);
		img.onerror = reject;
		img.src = src;
	});
}
