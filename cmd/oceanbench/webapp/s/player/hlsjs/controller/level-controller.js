/*
 * Level Controller
*/

import Event from '../events.js';
import EventHandler from '../hls-event-handler.js';
import { addGroupId, computeReloadInterval } from './level-helper.js';

const { performance } = window;
let chromeOrFirefox;

export default class LevelController extends EventHandler {
  constructor(hls) {
    super(hls,
      Event.MANIFEST_LOADED,
      Event.LEVEL_LOADED);

    this.canload = false;
    this.curLvlIdx = 0;
    this.manualLvlIdx = -1;
    this.timer = null;

    chromeOrFirefox = /chrome|firefox/.test(navigator.userAgent.toLowerCase());
  }

  onHandlerDestroying() {
    this.clearTimer();
    this.manualLvlIdx = -1;
  }

  clearTimer() {
    if (this.timer !== null) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }

  startLoad() {
    let levels = this._levels;

    this.canload = true;
    this.levelRetryCount = 0;

    // clean up live level details to force reload them, and reset load errors
    if (levels) {
      levels.forEach(level => {
        level.loadError = 0;
        const levelDetails = level.details;
        if (levelDetails && levelDetails.live) {
          level.details = undefined;
        }
      });
    }
    // speed up live playlist refresh if timer exists
    if (this.timer !== null) {
      this.loadLevel();
    }
  }

  stopLoad() {
    this.canload = false;
  }

  onManifestLoaded(data) {
    let levels = [];
    let audioTracks = [];
    let bitrateStart;
    let levelSet = {};
    let levelFromSet = null;
    let videoCodecFound = false;
    let audioCodecFound = false;

    // regroup redundant levels together
    data.levels.forEach(level => {
      const attributes = level.attrs;
      level.loadError = 0;
      level.fragmentError = false;

      videoCodecFound = videoCodecFound || !!level.videoCodec;
      audioCodecFound = audioCodecFound || !!level.audioCodec;

      levelFromSet = levelSet[level.bitrate]; // FIXME: we would also have to match the resolution here

      if (!levelFromSet) {
        level.url = [level.url];
        level.urlId = 0;
        levelSet[level.bitrate] = level;
        levels.push(level);
      } else {
        levelFromSet.url.push(level.url);
      }

      if (attributes) {
        if (attributes.AUDIO) {
          audioCodecFound = true;
          addGroupId(levelFromSet || level, 'audio', attributes.AUDIO);
        }
        if (attributes.SUBTITLES) {
          addGroupId(levelFromSet || level, 'text', attributes.SUBTITLES);
        }
      }
    });

    if (levels.length > 0) {
      // start bitrate is the first bitrate of the manifest
      bitrateStart = levels[0].bitrate;
      // sort level on bitrate
      levels.sort((a, b) => a.bitrate - b.bitrate);
      this._levels = levels;
      // find index of first level in sorted levels
      for (let i = 0; i < levels.length; i++) {
        if (levels[i].bitrate === bitrateStart) {
          this._firstLevel = i;
          break;
        }
      }

      // Audio is only alternate if manifest include a URI along with the audio group tag
      this.hls.trigger(Event.MANIFEST_PARSED, {
        levels,
        audioTracks,
        firstLevel: this._firstLevel,
        stats: data.stats,
        audio: audioCodecFound,
        video: videoCodecFound,
        altAudio: audioTracks.some(t => !!t.url)
      });
    } else {
      this.hls.trigger(Event.ERROR, {
        type: ErrorTypes.MEDIA_ERROR,
        details: ErrorDetails.MANIFEST_INCOMPATIBLE_CODECS_ERROR,
        fatal: true,
        url: this.hls.url,
        reason: 'no level with compatible codecs found in manifest'
      });
    }
  }

  get levels() {
    return this._levels;
  }

  get level() {
    return this.curLvlIdx;
  }

  set level(newLevel) {
    let levels = this._levels;
    if (levels) {
      newLevel = Math.min(newLevel, levels.length - 1);
      if (this.curLvlIdx !== newLevel || !levels[newLevel].details) {
        this.setLevelInternal(newLevel);
      }
    }
  }

  setLevelInternal(newLevel) {
    const levels = this._levels;
    const hls = this.hls;
    // check if level idx is valid
    if (newLevel >= 0 && newLevel < levels.length) {
      // stopping live reloading timer if any
      this.clearTimer();
      if (this.curLvlIdx !== newLevel) {
        console.log(`switching to level ${newLevel}`);
        this.curLvlIdx = newLevel;
        const levelProperties = levels[newLevel];
        levelProperties.level = newLevel;
        hls.trigger(Event.LEVEL_SWITCHING, levelProperties);
      }
      const level = levels[newLevel];
      const levelDetails = level.details;

      // check if we need to load playlist for this level
      if (!levelDetails || levelDetails.live) {
        // level not retrieved yet, or live playlist we need to (re)load it
        let urlId = level.urlId;
        hls.trigger(Event.LEVEL_LOADING, { url: level.url[urlId], level: newLevel, id: urlId });
      }
    } else {
      // invalid level id given, trigger error
      hls.trigger(Event.ERROR, {
        type: ErrorTypes.OTHER_ERROR,
        details: ErrorDetails.LEVEL_SWITCH_ERROR,
        level: newLevel,
        fatal: false,
        reason: 'invalid level idx'
      });
    }
  }

  get manualLevel() {
    return this.manualLvlIdx;
  }

  set manualLevel(newLevel) {
    this.manualLvlIdx = newLevel;
    if (this._startLevel === undefined) {
      this._startLevel = newLevel;
    }

    if (newLevel !== -1) {
      this.level = newLevel;
    }
  }

  get firstLevel() {
    return this._firstLevel;
  }

  set firstLevel(newLevel) {
    this._firstLevel = newLevel;
  }

  get startLevel() {
    // hls.startLevel takes precedence over config.startLevel
    // if none of these values are defined, fallback on this._firstLevel (first quality level appearing in variant manifest)
    if (this._startLevel === undefined) {
      let configStartLevel = this.hls.config.startLevel;
      if (configStartLevel !== undefined) {
        return configStartLevel;
      } else {
        return this._firstLevel;
      }
    } else {
      return this._startLevel;
    }
  }

  set startLevel(newLevel) {
    this._startLevel = newLevel;
  }

  onLevelLoaded(data) {
    const { level, details } = data;
    const curLevel = this._levels[level];
    // if current playlist is a live playlist, arm a timer to reload it
    if (details.live) {
      const reloadInterval = computeReloadInterval(curLevel.details, details, data.stats.trequest);
      console.log(`live playlist, reload in ${Math.round(reloadInterval)} ms`);
      this.timer = setTimeout(() => this.loadLevel(), reloadInterval);
    } else {
      this.clearTimer();
    }
  }

  loadLevel() {
    if (this.curLvlIdx !== null && this.canload) {
      const levelObject = this._levels[this.curLvlIdx];

      if (typeof levelObject === 'object' &&
        levelObject.url.length > 0) {
        const level = this.curLvlIdx;
        const id = levelObject.urlId;
        const url = levelObject.url[id];

        console.log(`Attempt loading level index ${level} with URL-id ${id}`);

        this.hls.trigger(Event.LEVEL_LOADING, { url, level, id });
      }
    }
  }

  get nextLoadLevel() {
    if (this.manualLvlIdx !== -1) {
      return this.manualLvlIdx;
    } else {
      return this.hls.nextAutoLevel;
    }
  }

  set nextLoadLevel(nextLevel) {
    this.level = nextLevel;
    if (this.manualLvlIdx === -1) {
      this.hls.nextAutoLevel = nextLevel;
    }
  }
}
