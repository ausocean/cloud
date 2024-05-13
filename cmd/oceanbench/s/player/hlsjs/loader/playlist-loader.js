/*
AUTHOR
  Trek Hopton <trek@ausocean.org>

LICENSE
  This file is Copyright (C) 2020 the Australian Ocean Lab (AusOcean)

  It is free software: you can redistribute it and/or modify them
  under the terms of the GNU General Public License as published by the
  Free Software Foundation, either version 3 of the License, or (at your
  option) any later version.

  It is distributed in the hope that it will be useful, but WITHOUT
  ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
  FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License
  for more details.

  You should have received a copy of the GNU General Public License in gpl.txt.
  If not, see http://www.gnu.org/licenses.

  For hls.js Copyright notice and license, see LICENSE file.
*/

import { PlaylistContextType, PlaylistLevelType } from '../types/loader.js';
import Event from '../events.js';
import EventHandler from '../hls-event-handler.js';
import M3U8Parser from './m3u8-parser.js';

const { performance } = window;

class PlaylistLoader extends EventHandler {
  constructor(hls) {
    super(hls,
      Event.MANIFEST_LOADING,
      Event.LEVEL_LOADING,
      Event.AUDIO_TRACK_LOADING,
      Event.SUBTITLE_TRACK_LOADING);
    this.hls = hls;
    this.loaders = {};
  }

  /**
 * @param {PlaylistContextType} type
 * @returns {boolean}
 */
  static canHaveQualityLevels(type) {
    return (type !== PlaylistContextType.AUDIO_TRACK &&
      type !== PlaylistContextType.SUBTITLE_TRACK);
  }

  /**
 * Map context.type to LevelType
 * @param {PlaylistLoaderContext} context
 * @returns {LevelType}
 */
  static mapContextToLevelType(context) {
    const { type } = context;

    switch (type) {
      case PlaylistContextType.AUDIO_TRACK:
        return PlaylistLevelType.AUDIO;
      case PlaylistContextType.SUBTITLE_TRACK:
        return PlaylistLevelType.SUBTITLE;
      default:
        return PlaylistLevelType.MAIN;
    }
  }

  static getResponseUrl(response, context) {
    let url = response.url;
    // responseURL not supported on some browsers (it is used to detect URL redirection)
    // data-uri mode also not supported (but no need to detect redirection)
    if (url === undefined || url.indexOf('data:') === 0) {
      // fallback to initial URL
      url = context.url;
    }
    return url;
  }

  /**
   * Returns defaults or configured loader-type overloads (pLoader and loader config params)
   * Default loader is XHRLoader (see utils)
   * @param {PlaylistLoaderContext} context
   * @returns {Loader} or other compatible configured overload
   */
  createInternalLoader(context) {
    const config = this.hls.config;
    const PLoader = config.pLoader;
    const Loader = config.loader;
    // TODO(typescript-config): Verify once config is typed that InternalLoader always returns a Loader
    const InternalLoader = PLoader || Loader;

    const loader = new InternalLoader(config);

    // TODO - Do we really need to assign the instance or if the dep has been lost
    context.loader = loader;
    this.loaders[context.type] = loader;

    return loader;
  }

  getInternalLoader(context) {
    return this.loaders[context.type];
  }

  resetInternalLoader(contextType) {
    if (this.loaders[contextType]) {
      delete this.loaders[contextType];
    }
  }

  onManifestLoading(data) {
    this.load({
      url: data.url,
      type: PlaylistContextType.MANIFEST,
      level: 0,
      id: null,
      responseType: 'text'
    });
  }

  onLevelLoading(data) {
    this.load({
      url: data.url,
      type: PlaylistContextType.LEVEL,
      level: data.level,
      id: data.id,
      responseType: 'text'
    });
  }

  onAudioTrackLoading(data) {
    this.load({
      url: data.url,
      type: PlaylistContextType.AUDIO_TRACK,
      level: null,
      id: data.id,
      responseType: 'text'
    });
  }

  onSubtitleTrackLoading(data) {
    this.load({
      url: data.url,
      type: PlaylistContextType.SUBTITLE_TRACK,
      level: null,
      id: data.id,
      responseType: 'text'
    });
  }

  load(context) {
    const config = this.hls.config;

    // Check if a loader for this context already exists
    let loader = this.getInternalLoader(context);
    if (loader) {
      const loaderContext = loader.context;
      if (loaderContext && loaderContext.url === context.url) { // same URL can't overlap
        return false;
      } else {
        console.warn(`aborting previous loader for type: ${context.type}`);
        loader.abort();
      }
    }

    let maxRetry;
    let timeout;
    let retryDelay;
    let maxRetryDelay;

    // apply different configs for retries depending on
    // context (manifest, level, audio/subs playlist)
    switch (context.type) {
      case PlaylistContextType.MANIFEST:
        maxRetry = config.manifestLoadingMaxRetry;
        timeout = config.manifestLoadingTimeOut;
        retryDelay = config.manifestLoadingRetryDelay;
        maxRetryDelay = config.manifestLoadingMaxRetryTimeout;
        break;
      case PlaylistContextType.LEVEL:
        // Disable internal loader retry logic, since we are managing retries in Level Controller
        maxRetry = 0;
        maxRetryDelay = 0;
        retryDelay = 0;
        timeout = config.levelLoadingTimeOut;
        // TODO Introduce retry settings for audio-track and subtitle-track, it should not use level retry config
        break;
      default:
        maxRetry = config.levelLoadingMaxRetry;
        timeout = config.levelLoadingTimeOut;
        retryDelay = config.levelLoadingRetryDelay;
        maxRetryDelay = config.levelLoadingMaxRetryTimeout;
        break;
    }

    loader = this.createInternalLoader(context);

    const loaderConfig = {
      timeout,
      maxRetry,
      retryDelay,
      maxRetryDelay
    };

    const loaderCallbacks = {
      onSuccess: this.loadsuccess.bind(this),
      onError: this.loaderror.bind(this),
      onTimeout: this.loadtimeout.bind(this)
    };

    loader.load(context, loaderConfig, loaderCallbacks);

    return true;
  }

  loadsuccess(response, stats, context, networkDetails = null) {
    if (context.isSidxRequest) {
      this._handleSidxRequest(response, context);
      this._handlePlaylistLoaded(response, stats, context, networkDetails);
      return;
    }

    this.resetInternalLoader(context.type);
    if (typeof response.data !== 'string') {
      throw new Error('expected responseType of "text" for PlaylistLoader');
    }

    const string = response.data;

    stats.tload = performance.now();

    // Validate if it is an M3U8 at all
    if (string.indexOf('#EXTM3U') !== 0) {
      console.error("no EXTM3U delimiter");
      return;
    }

    // Check if chunk-list or master. handle empty chunk list case (first EXTINF not signaled, but TARGETDURATION present)
    if (string.indexOf('#EXTINF:') > 0 || string.indexOf('#EXT-X-TARGETDURATION:') > 0) {
      this._handleTrackOrLevelPlaylist(response, stats, context, networkDetails);
    } else {
      console.log("handling of master playlists is not implemented");
      // this._handleMasterPlaylist(response, stats, context, networkDetails);
    }

  }

  loaderror(response, context, networkDetails = null) {
    console.error("network error while loading", response);
  }

  loadtimeout(stats, context, networkDetails = null) {
    console.error("network timeout while loading", stats);
  }

  _handleTrackOrLevelPlaylist(response, stats, context, networkDetails) {
    const hls = this.hls;

    const { id, level, type } = context;

    const url = PlaylistLoader.getResponseUrl(response, context);

    // if the values are null, they will result in the else conditional
    const levelUrlId = Number.isFinite(id) ? id : 0;
    const levelId = Number.isFinite(level) ? level : levelUrlId;

    const levelType = PlaylistLoader.mapContextToLevelType(context);
    const levelDetails = M3U8Parser.parseLevelPlaylist(response.data, url, levelId, levelType, levelUrlId);

    // set stats on level structure
    // TODO(jstackhouse): why? mixing concerns, is it just treated as value bag?
    (levelDetails).tload = stats.tload;

    // We have done our first request (Manifest-type) and receive
    // not a master playlist but a chunk-list (track/level)
    // We fire the manifest-loaded event anyway with the parsed level-details
    // by creating a single-level structure for it.
    if (type === PlaylistContextType.MANIFEST) {
      const singleLevel = {
        url,
        details: levelDetails
      };

      hls.trigger(Event.MANIFEST_LOADED, {
        levels: [singleLevel],
        audioTracks: [],
        url,
        stats,
        networkDetails
      });
    }

    // save parsing time
    stats.tparsed = performance.now();

    // extend the context with the new levelDetails property
    context.levelDetails = levelDetails;

    this._handlePlaylistLoaded(response, stats, context, networkDetails);
  }

  _handlePlaylistLoaded(response, stats, context, networkDetails) {
    const { type, level, id, levelDetails } = context;

    if (!levelDetails || !levelDetails.targetduration) {
      console.error("manifest parsing error");
      return;
    }

    const canHaveLevels = PlaylistLoader.canHaveQualityLevels(context.type);
    if (canHaveLevels) {
      this.hls.trigger(Event.LEVEL_LOADED, {
        details: levelDetails,
        level: level || 0,
        id: id || 0,
        stats,
        networkDetails
      });
    } else {
      switch (type) {
        case PlaylistContextType.AUDIO_TRACK:
          this.hls.trigger(Event.AUDIO_TRACK_LOADED, {
            details: levelDetails,
            id,
            stats,
            networkDetails
          });
          break;
        case PlaylistContextType.SUBTITLE_TRACK:
          this.hls.trigger(Event.SUBTITLE_TRACK_LOADED, {
            details: levelDetails,
            id,
            stats,
            networkDetails
          });
          break;
      }
    }
  }
}

export default PlaylistLoader;
