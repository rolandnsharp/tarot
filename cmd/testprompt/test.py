#!/usr/bin/env python3
"""Generate a card image and show it as 24x40 hex in terminal."""

import os
import sys

os.environ["HF_HUB_ENABLE_HF_TRANSFER"] = "0"

import torch
from diffusers import AutoPipelineForText2Image
from PIL import Image

def render_hex(img, width=24, height=40):
    """Downscale and render as half-block pixel art in terminal."""
    img = img.convert("RGB").resize((width, height), Image.LANCZOS)
    for row in range(0, height - 1, 2):
        for col in range(width):
            r1, g1, b1 = img.getpixel((col, row))
            r2, g2, b2 = img.getpixel((col, row + 1))
            print(f"\033[38;2;{r1};{g1};{b1}m\033[48;2;{r2};{g2};{b2}m▀\033[0m", end="")
        print()

prompt = sys.argv[1] if len(sys.argv) > 1 else "tarot card"

print("Loading model...")
pipe = AutoPipelineForText2Image.from_pretrained(
    "stabilityai/sdxl-turbo", torch_dtype=torch.float32
)
pipe.to("cpu")

print(f"Prompt: {prompt}")
print("Generating...")
image = pipe(
    prompt=prompt,
    num_inference_steps=4,
    guidance_scale=0.0,
    width=512,
    height=768,
).images[0]

# Save full PNG for reference
image.save("/tmp/tarot-test.png")
print("Full image: /tmp/tarot-test.png")
print()

# Show at target resolution
print("At 24×40:")
render_hex(image)
