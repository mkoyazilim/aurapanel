#!/usr/bin/env python3
"""
Prepare pinned OpenLiteSpeed assets for AuraPanel mirror.

Outputs:
  - openlitespeed-manifest.env
  - pinned package files (.deb/.rpm)
"""

from __future__ import annotations

import argparse
import bz2
import datetime
import gzip
import hashlib
import io
import lzma
import os
import re
import sys
import urllib.request
import xml.etree.ElementTree as ET
from dataclasses import dataclass
from pathlib import Path
from typing import Iterable


DEB_CODENAMES = ("jammy", "noble", "bookworm")
RPM_EL_VERSIONS = ("8", "9")


def fetch_bytes(url: str) -> bytes:
    with urllib.request.urlopen(url, timeout=60) as response:  # nosec B310
        return response.read()


def decompress_payload(payload: bytes, resource_name: str) -> bytes:
    lower = resource_name.lower()
    if lower.endswith(".gz"):
        return gzip.decompress(payload)
    if lower.endswith(".xz"):
        return lzma.decompress(payload)
    if lower.endswith(".bz2"):
        return bz2.decompress(payload)
    if lower.endswith(".zst"):
        try:
            import zstandard  # type: ignore
        except Exception as exc:  # pylint: disable=broad-except
            raise RuntimeError(
                "zstd metadata detected but 'zstandard' module is missing. "
                "Install it with: python3 -m pip install zstandard"
            ) from exc
        dctx = zstandard.ZstdDecompressor()
        with dctx.stream_reader(io.BytesIO(payload)) as reader:
            return reader.read()
    return payload


def sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as fh:
        for chunk in iter(lambda: fh.read(1024 * 1024), b""):
            h.update(chunk)
    return h.hexdigest()


def normalize_version_key(version: str) -> list[object]:
    parts = re.split(r"([0-9]+)", version)
    key: list[object] = []
    for part in parts:
        if part == "":
            continue
        if part.isdigit():
            key.append(int(part))
        else:
            key.append(part)
    return key


@dataclass
class PackageAsset:
    key_prefix: str
    version: str
    url: str
    sha256: str
    local_file: Path


def parse_deb_assets(output_dir: Path) -> list[PackageAsset]:
    assets: list[PackageAsset] = []
    for codename in DEB_CODENAMES:
        packages_url = (
            f"https://rpms.litespeedtech.com/debian/dists/{codename}/main/binary-amd64/Packages.gz"
        )
        raw = fetch_bytes(packages_url)
        content = decompress_payload(raw, packages_url).decode("utf-8", errors="replace")

        candidates: list[dict[str, str]] = []
        for block in content.split("\n\n"):
            fields: dict[str, str] = {}
            for line in block.splitlines():
                if ": " not in line:
                    continue
                k, v = line.split(": ", 1)
                fields[k.strip()] = v.strip()
            if fields.get("Package") != "openlitespeed":
                continue
            if fields.get("Architecture") != "amd64":
                continue
            if "Filename" not in fields:
                continue
            candidates.append(fields)

        if not candidates:
            raise RuntimeError(f"openlitespeed package not found in {packages_url}")

        candidates.sort(key=lambda item: normalize_version_key(item.get("Version", "")))
        pkg = candidates[-1]
        rel_path = pkg["Filename"].lstrip("/")
        url = f"https://rpms.litespeedtech.com/debian/{rel_path}"
        version = pkg.get("Version", "unknown")
        filename = Path(rel_path).name
        local_path = output_dir / "pinned" / filename
        local_path.parent.mkdir(parents=True, exist_ok=True)
        local_path.write_bytes(fetch_bytes(url))
        sha = sha256_file(local_path)
        key_prefix = f"AURAPANEL_OLS_DEB_{codename.upper()}_AMD64"
        assets.append(
            PackageAsset(
                key_prefix=key_prefix,
                version=version,
                url=f"{{MIRROR_BASE}}/deps/litespeed/pinned/{filename}",
                sha256=sha,
                local_file=local_path,
            )
        )
    return assets


def parse_rpm_assets(output_dir: Path) -> list[PackageAsset]:
    assets: list[PackageAsset] = []
    ns_repo = {"repo": "http://linux.duke.edu/metadata/repo"}
    ns_common = {"common": "http://linux.duke.edu/metadata/common"}

    for el_ver in RPM_EL_VERSIONS:
        base = f"https://rpms.litespeedtech.com/centos/{el_ver}/x86_64"
        repomd_url = f"{base}/repodata/repomd.xml"
        repomd_root = ET.fromstring(fetch_bytes(repomd_url))

        primary_href = ""
        for data in repomd_root.findall("repo:data", ns_repo):
            if data.attrib.get("type") != "primary":
                continue
            location = data.find("repo:location", ns_repo)
            if location is None:
                continue
            primary_href = location.attrib.get("href", "")
            if primary_href:
                break
        if not primary_href:
            raise RuntimeError(f"primary metadata not found in {repomd_url}")

        primary_url = f"{base}/{primary_href.lstrip('/')}"
        primary_raw = decompress_payload(fetch_bytes(primary_url), primary_href)
        primary_root = ET.fromstring(primary_raw)

        candidates: list[tuple[list[object], ET.Element]] = []
        for pkg in primary_root.findall("common:package", ns_common):
            name = pkg.findtext("common:name", default="", namespaces=ns_common)
            arch = pkg.findtext("common:arch", default="", namespaces=ns_common)
            if name != "openlitespeed" or arch != "x86_64":
                continue
            version_node = pkg.find("common:version", ns_common)
            if version_node is None:
                continue
            version = version_node.attrib.get("ver", "")
            release = version_node.attrib.get("rel", "")
            candidates.append((normalize_version_key(f"{version}-{release}"), pkg))

        if not candidates:
            raise RuntimeError(f"openlitespeed package not found in {primary_url}")

        candidates.sort(key=lambda item: item[0])
        pkg = candidates[-1][1]
        version_node = pkg.find("common:version", ns_common)
        assert version_node is not None
        version = f"{version_node.attrib.get('ver', '')}-{version_node.attrib.get('rel', '')}"
        location = pkg.find("common:location", ns_common)
        if location is None:
            raise RuntimeError(f"missing package location in {primary_url}")
        rel_path = location.attrib.get("href", "").lstrip("/")
        if not rel_path:
            raise RuntimeError(f"empty package location in {primary_url}")

        url = f"{base}/{rel_path}"
        filename = Path(rel_path).name
        local_path = output_dir / "pinned" / filename
        local_path.parent.mkdir(parents=True, exist_ok=True)
        local_path.write_bytes(fetch_bytes(url))
        sha = sha256_file(local_path)
        key_prefix = f"AURAPANEL_OLS_RPM_EL{el_ver}_X86_64"
        assets.append(
            PackageAsset(
                key_prefix=key_prefix,
                version=version,
                url=f"{{MIRROR_BASE}}/deps/litespeed/pinned/{filename}",
                sha256=sha,
                local_file=local_path,
            )
        )

    return assets


def write_manifest(output_dir: Path, assets: Iterable[PackageAsset]) -> None:
    assets = list(assets)
    manifest = output_dir / "openlitespeed-manifest.env"
    latest = ""
    if assets:
        latest = sorted(assets, key=lambda item: normalize_version_key(item.version))[-1].version

    lines = [
        "# Generated by scripts/prepare_ols_mirror_assets.py",
        "# {MIRROR_BASE} token is replaced in mirror-sync workflow.",
        f"AURAPANEL_OLS_PINNED_GENERATED_AT={datetime.datetime.now(datetime.timezone.utc).isoformat()}",
        f"AURAPANEL_OLS_PINNED_LATEST_VERSION={latest}",
    ]
    for item in assets:
        lines.append(f"{item.key_prefix}_VERSION={item.version}")
        lines.append(f"{item.key_prefix}_URL={item.url}")
        lines.append(f"{item.key_prefix}_SHA256={item.sha256}")
    manifest.write_text("\n".join(lines) + "\n", encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser(description="Prepare pinned OLS mirror assets.")
    parser.add_argument(
        "--output",
        required=True,
        help="Output directory (example: dist/mirror/deps/litespeed)",
    )
    args = parser.parse_args()

    output_dir = Path(args.output).resolve()
    output_dir.mkdir(parents=True, exist_ok=True)
    (output_dir / "pinned").mkdir(parents=True, exist_ok=True)

    deb_assets = parse_deb_assets(output_dir)
    rpm_assets = parse_rpm_assets(output_dir)
    all_assets = deb_assets + rpm_assets
    write_manifest(output_dir, all_assets)
    print(f"Prepared {len(all_assets)} OpenLiteSpeed pinned assets in {output_dir}")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:  # pylint: disable=broad-except
        print(f"ERROR: {exc}", file=sys.stderr)
        raise SystemExit(1)
