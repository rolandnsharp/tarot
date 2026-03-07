#!/usr/bin/env python3
"""Generate a full tarot deck using Stable Diffusion (SDXL-Turbo, CPU)."""

import os
import sys
import time
import re
from pathlib import Path

os.environ["HF_HUB_ENABLE_HF_TRANSFER"] = "0"

import torch
from diffusers import AutoPipelineForText2Image
from PIL import Image

STYLE = (
    "art nouveau style illustration, ornate gold border, "
    "rich jewel tones, highly detailed, vertical portrait composition, "
    "Alphonse Mucha inspired, decorative botanical framing"
)

MAJOR_ARCANA = [
    (0, "The Fool", "a young traveler stepping off a cliff edge with a small white dog at their heels, carrying a bindle, sun shining behind"),
    (1, "The Magician", "a robed figure at a table with a wand raised high, cups swords and pentacles on the table, infinity symbol above head"),
    (2, "The High Priestess", "a serene woman seated between two pillars, crescent moon at her feet, scroll in her lap, veil behind her"),
    (3, "The Empress", "a regal woman in flowing robes seated in a lush garden, wheat field, crown of twelve stars, abundance"),
    (4, "The Emperor", "a stern ruler on a stone throne with ram heads, red robes, orb and scepter, mountains behind"),
    (5, "The Hierophant", "a religious figure on a throne between two pillars, two acolytes kneeling, triple crown, raised hand blessing"),
    (6, "The Lovers", "a man and woman beneath an angel with spread wings, tree of knowledge, tree of fire, garden of eden"),
    (7, "The Chariot", "a warrior in armor driving a chariot pulled by two sphinxes one black one white, city walls behind, stars above"),
    (8, "Strength", "a woman gently closing the jaws of a lion, infinity symbol above her head, flowers in her hair, calm expression"),
    (9, "The Hermit", "a cloaked old man on a mountain peak holding a lantern with a six-pointed star, staff in other hand, solitude"),
    (10, "Wheel of Fortune", "a great wheel with mystical symbols turning in clouds, sphinx atop, serpent descending, Anubis ascending"),
    (11, "Justice", "a figure seated between two pillars holding a sword upright in right hand and scales in left, crown, red robe"),
    (12, "The Hanged Man", "a man suspended upside down from a living tree by one foot, halo around head, serene expression, crossed leg"),
    (13, "Death", "a skeleton knight in black armor on a white horse, fallen king, bishop praying, rising sun between two towers"),
    (14, "Temperance", "an angel with one foot on land one in water, pouring water between two cups, sun on horizon, irises growing"),
    (15, "The Devil", "a horned demon on a pedestal with two chained naked figures, inverted pentagram, dark throne, bat wings"),
    (16, "The Tower", "a tall tower struck by lightning, crown blown off top, two figures falling, flames, dark stormy sky"),
    (17, "The Star", "a naked woman kneeling by a pool pouring water from two jugs, eight-pointed stars in sky, ibis in tree"),
    (18, "The Moon", "a full moon with a face between two towers, a dog and wolf howling, crayfish emerging from water, winding path"),
    (19, "The Sun", "a joyful naked child on a white horse, large sun with face, sunflowers, red banner, stone wall"),
    (20, "Judgement", "an angel blowing a trumpet in clouds, naked figures rising from coffins below, mountains, cross on banner"),
    (21, "The World", "a dancing figure wrapped in a purple sash inside a laurel wreath, four creatures in corners: angel eagle bull lion"),
]

SUITS = {
    "wands": "wooden wand or staff",
    "cups": "golden chalice or cup",
    "swords": "silver sword",
    "pentacles": "golden coin with pentacle star",
}

COURT_CARDS = {
    11: ("Page", "a young person"),
    12: ("Knight", "an armored knight on horseback"),
    13: ("Queen", "a queen seated on a throne"),
    14: ("King", "a king seated on a throne"),
}

MINOR_DESCRIPTIONS = {
    1: "a single {symbol} held by a hand emerging from a cloud, rays of light",
    2: "a figure holding two {symbols}, contemplating a choice",
    3: "three {symbols} arranged with a figure in contemplation",
    4: "four {symbols} arranged symmetrically, a figure at rest",
    5: "five {symbols} in a scene of conflict or struggle between figures",
    6: "six {symbols} with figures in a scene of giving or receiving",
    7: "seven {symbols} with a figure facing a challenge or making a stand",
    8: "eight {symbols} arranged dynamically suggesting swift movement",
    9: "nine {symbols} with a solitary figure showing resilience",
    10: "ten {symbols} with figures bearing a heavy burden or great abundance",
}


def slugify(name):
    name = name.lower().replace(" ", "-")
    return re.sub(r"[^a-z0-9-]", "", name)



def generate_card(pipe, prompt, hex_path):
    """Generate a single card image and save as small hex."""
    full_prompt = f"{prompt}, {STYLE}"
    image = pipe(
        prompt=full_prompt,
        num_inference_steps=4,
        guidance_scale=0.0,
        width=512,
        height=768,
    ).images[0]
    png_to_hex_from_image(image, hex_path, width=24, height=40)


def png_to_hex_from_image(img, hex_path, width, height):
    """Downscale a PIL Image and save as hex CSV format."""
    img = img.convert("RGB").resize((width, height), Image.LANCZOS)
    lines = []
    for y in range(height):
        row = []
        for x in range(width):
            r, g, b = img.getpixel((x, y))
            row.append(f"{r:02x}{g:02x}{b:02x}")
        lines.append(",".join(row))
    with open(hex_path, "w") as f:
        f.write("\n".join(lines) + "\n")


def main():
    out_dir = Path("decks/diffusion-sm")
    major_dir = out_dir / "major"
    minor_dir = out_dir / "minor"

    for d in [major_dir, minor_dir]:
        d.mkdir(parents=True, exist_ok=True)

    # Check for resume - skip already generated hex files
    existing = set()
    for f in major_dir.glob("*.hex"):
        existing.add(f.name)
    for f in minor_dir.glob("*.hex"):
        existing.add(f.name)
    if (out_dir / "back.hex").exists():
        existing.add("back.hex")

    print("Loading SDXL-Turbo model...")
    t0 = time.time()
    pipe = AutoPipelineForText2Image.from_pretrained(
        "stabilityai/sdxl-turbo",
        torch_dtype=torch.float32,
    )
    pipe.to("cpu")
    print(f"Model loaded in {time.time() - t0:.1f}s")

    total = 22 + 56 + 1  # major + minor + back
    done = 0

    # Major Arcana
    for num, name, desc in MAJOR_ARCANA:
        hex_name = f"{num:02d}-{slugify(name)}.hex"
        if hex_name in existing:
            print(f"  Skipping {name} (already exists)")
            done += 1
            continue

        print(f"\n[{done+1}/{total}] Generating {name}...")
        t1 = time.time()
        generate_card(
            pipe,
            f"Tarot card {name}: {desc}",
            major_dir / hex_name,
        )
        print(f"  Done in {time.time() - t1:.1f}s")
        done += 1

    # Minor Arcana
    for suit_name, symbol in SUITS.items():
        symbols = symbol + "s"
        for num in range(1, 15):
            if num <= 10:
                card_name = f"{_number_name(num)} of {suit_name.title()}"
                desc = MINOR_DESCRIPTIONS[num].format(symbol=symbol, symbols=symbols)
            else:
                court_title, court_desc = COURT_CARDS[num]
                card_name = f"{court_title} of {suit_name.title()}"
                desc = f"{court_desc} holding a {symbol}, {suit_name} suit imagery"

            hex_name = f"{suit_name}-{num:02d}.hex"
            if hex_name in existing:
                print(f"  Skipping {card_name} (already exists)")
                done += 1
                continue

            print(f"\n[{done+1}/{total}] Generating {card_name}...")
            t1 = time.time()
            generate_card(
                pipe,
                f"Tarot card {card_name}: {desc}",
                minor_dir / hex_name,
            )
            print(f"  Done in {time.time() - t1:.1f}s")
            done += 1

    # Card back
    back_hex = "back.hex"
    if back_hex not in existing:
        print(f"\n[{done+1}/{total}] Generating card back...")
        t1 = time.time()
        generate_card(
            pipe,
            "Ornate tarot card back design, intricate geometric mandala pattern, "
            "deep purple and gold, art nouveau scrollwork border, symmetrical",
            out_dir / back_hex,
        )
        print(f"  Done in {time.time() - t1:.1f}s")

    print(f"\nDeck generation complete! {done} cards in {out_dir}/")


def _number_name(n):
    return [
        "", "Ace", "Two", "Three", "Four", "Five",
        "Six", "Seven", "Eight", "Nine", "Ten",
    ][n]


if __name__ == "__main__":
    main()
