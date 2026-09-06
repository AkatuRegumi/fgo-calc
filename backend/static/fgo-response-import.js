(function (root) {
    'use strict';

    function stripBom(value) {
        return String(value ?? '').replace(/^\uFEFF/, '').trim();
    }

    function parseJsonCandidate(text) {
        try {
            const value = JSON.parse(text);
            if (typeof value === 'string') return JSON.parse(value);
            return value;
        } catch (_) {
            return null;
        }
    }

    function decodeBase64Utf8(text) {
        let normalized = stripBom(text).replace(/\s+/g, '').replace(/-/g, '+').replace(/_/g, '/');
        if (!normalized) throw new Error('响应内容为空');
        normalized += '='.repeat((4 - (normalized.length % 4)) % 4);

        if (typeof Buffer !== 'undefined' && typeof Buffer.from === 'function') {
            return Buffer.from(normalized, 'base64').toString('utf8');
        }
        if (typeof root.atob !== 'function') throw new Error('当前环境不支持 Base64 解码');
        const binary = root.atob(normalized);
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
        if (typeof TextDecoder !== 'undefined') {
            return new TextDecoder('utf-8', {fatal: true}).decode(bytes);
        }
        let escaped = '';
        for (const byte of bytes) escaped += '%' + byte.toString(16).padStart(2, '0');
        return decodeURIComponent(escaped);
    }

    function unwrapResponseText(rawText) {
        const source = stripBom(rawText);
        if (!source) throw new Error('响应文件为空');

        const direct = parseJsonCandidate(source);
        if (direct && typeof direct === 'object') return direct;

        const candidates = [];
        // Some sniffers export a query-style "response=<payload>" wrapper.
        const match = source.match(/^(?:response|data|body|payload)=([\s\S]+)$/i);
        if (match) candidates.push(match[1]);
        candidates.push(source);

        for (const candidateRaw of candidates) {
            const variants = [stripBom(candidateRaw)];
            if (candidateRaw.includes('%')) {
                try { variants.unshift(stripBom(decodeURIComponent(candidateRaw))); } catch (_) {}
            }
            for (const candidate of variants) {
                const parsed = parseJsonCandidate(candidate);
                if (parsed && typeof parsed === 'object') return parsed;
                try {
                    const decoded = decodeBase64Utf8(candidate);
                    const decodedJson = parseJsonCandidate(stripBom(decoded));
                    if (decodedJson && typeof decodedJson === 'object') return decodedJson;
                } catch (_) {}
            }
        }
        throw new Error('无法识别响应格式；支持原始 JSON、URL 编码文本或 Base64 登录响应');
    }

    function asArray(value) {
        return Array.isArray(value) ? value : [];
    }

    function toInt(value, fallback = 0) {
        const n = Number.parseInt(String(value ?? ''), 10);
        return Number.isFinite(n) ? n : fallback;
    }

    function mergedCacheArray(cache, key, identityKey) {
        const replaced = asArray(cache?.replaced?.[key]);
        const updated = asArray(cache?.updated?.[key]);
        if (!updated.length) return replaced.slice();
        if (!replaced.length) return updated.slice();
        const output = new Map();
        for (const item of replaced) {
            const identity = String(item?.[identityKey] ?? '');
            if (identity) output.set(identity, item);
        }
        for (const item of updated) {
            const identity = String(item?.[identityKey] ?? '');
            if (identity) output.set(identity, item);
        }
        return Array.from(output.values());
    }

    function extractSnapshot(payload, options = {}) {
        if (!payload || typeof payload !== 'object') throw new Error('登录响应不是 JSON 对象');
        const cache = payload.cache;
        if (!cache || typeof cache !== 'object') throw new Error('登录响应缺少 cache 数据');

        const servantIds = new Set(Array.from(options.servantIds || [], value => Number(value)).filter(Number.isInteger));
        const craftEssenceIds = new Set(Array.from(options.craftEssenceIds || [], value => Number(value)).filter(Number.isInteger));
        if (!servantIds.size) throw new Error('本地从者主数据为空，无法匹配 Box');

        const collection = mergedCacheArray(cache, 'userSvtCollection', 'svtId');
        if (!collection.length) throw new Error('登录响应中没有 userSvtCollection；请抓取完整 toplogin 响应');

        const servants = [];
        let unknownOwnedServants = 0;
        for (const item of collection) {
            const id = toInt(item?.svtId, -1);
            if (!servantIds.has(id)) continue;
            if (toInt(item?.status, 0) !== 2) continue;
            const bondRank = Math.max(0, toInt(item?.friendshipRank, 0));
            const bondTotal = Math.max(0, toInt(item?.friendship, 0));
            const exceedCount = Math.max(0, toInt(item?.friendshipExceedCount, 0));
            const bondRankMax = Math.max(10, 10 + exceedCount, bondRank);
            servants.push({id, bondRank, bondRankMax, bondTotal, exceedCount});
        }
        for (const item of collection) {
            const id = toInt(item?.svtId, -1);
            if (toInt(item?.status, 0) === 2 && id > 0 && id % 100 === 0 && !servantIds.has(id) && id < 9000000) unknownOwnedServants++;
        }
        if (!servants.length) throw new Error('没有匹配到当前数据库中的已持有从者');

        const liveInventory = [
            ...mergedCacheArray(cache, 'userSvt', 'id'),
            ...mergedCacheArray(cache, 'userSvtStorage', 'id')
        ];
        const ceInventoryAvailable = liveInventory.length > 0;
        const mlbCraftEssences = new Set();
        const nonMlbCraftEssences = new Set();
        if (ceInventoryAvailable && craftEssenceIds.size) {
            for (const item of liveInventory) {
                const id = toInt(item?.svtId, -1);
                if (!craftEssenceIds.has(id)) continue;
                if (toInt(item?.limitCount, 0) >= 4) mlbCraftEssences.add(id);
                else nonMlbCraftEssences.add(id);
            }
            for (const id of mlbCraftEssences) nonMlbCraftEssences.delete(id);
        }

        const grandRecords = mergedCacheArray(cache, 'userSvtGrand', 'grandGraphId')
            .map(item => ({
                grandGraphId: toInt(item?.grandGraphId, 0),
                svtId: toInt(item?.svtId, 0)
            }))
            .filter(item => item.svtId > 0);

        return {
            serverTime: Math.max(0, toInt(cache.serverTime, 0)),
            servants: servants.sort((a, b) => a.id - b.id),
            mlbCraftEssences: Array.from(mlbCraftEssences).sort((a, b) => a - b),
            nonMlbCraftEssences: Array.from(nonMlbCraftEssences).sort((a, b) => a - b),
            ceInventoryAvailable,
            unknownOwnedServants,
            grandRecords
        };
    }

    const api = {unwrapResponseText, extractSnapshot, decodeBase64Utf8};
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    root.FgoResponseImport = api;
})(typeof globalThis !== 'undefined' ? globalThis : this);
