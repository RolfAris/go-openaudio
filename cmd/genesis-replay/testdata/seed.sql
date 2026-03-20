-- Test seed data for genesis-replay verification.
-- Covers: users, tracks (with remix), album, playlist, follows, reposts,
--         saves, plays, comments (with reply), comment reactions, tip reactions.
--
-- ID ranges mirror production offsets:
--   users:     >= 3,000,000
--   tracks:    >= 2,000,000
--   playlists: >= 400,000
--   comments:  >= 4,000,000
--
-- All rows share a single block so there is only one FK dependency to satisfy.

-- ============================================================
-- BLOCK (anchor for all FK references)
-- ============================================================

INSERT INTO blocks (blockhash, number, parenthash, is_current) VALUES
  ('0x0000000000000000000000000000000000000000000000000000000000000001', 1, '0x0000000000000000000000000000000000000000000000000000000000000000', true);

-- ============================================================
-- USERS
-- user1 = artist (has tracks, album)
-- user2 = artist (has tracks)
-- user3 = artist (has tracks, is remixed by user2)
-- user4 = fan (follows, saves, reposts, comments)
-- user5 = fan (follows, reacts)
-- ============================================================

INSERT INTO users (
  blockhash, blocknumber, user_id, is_current, txhash,
  handle, handle_lc, wallet, name, bio, location,
  is_verified, is_deactivated, is_available, is_storage_v2,
  allow_ai_attribution, created_at, updated_at
) VALUES
  (
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    3000001, true, '0xaaa0000000000000000000000000000000000000000000000000000000000001',
    'artist_one', 'artist_one', '0xAAAA000000000000000000000000000000000001',
    'Artist One', 'Electronic producer based in LA', 'Los Angeles, CA',
    false, false, true, true, false,
    '2021-01-01 00:00:00', '2021-01-01 00:00:00'
  ),
  (
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    3000002, true, '0xaaa0000000000000000000000000000000000000000000000000000000000002',
    'artist_two', 'artist_two', '0xAAAA000000000000000000000000000000000002',
    'Artist Two', 'Indie rock and folk', 'Nashville, TN',
    false, false, true, true, false,
    '2021-02-01 00:00:00', '2021-02-01 00:00:00'
  ),
  (
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    3000003, true, '0xaaa0000000000000000000000000000000000000000000000000000000000003',
    'artist_three', 'artist_three', '0xAAAA000000000000000000000000000000000003',
    'Artist Three', 'Jazz and soul', 'New York, NY',
    true, false, true, true, false,
    '2021-03-01 00:00:00', '2021-03-01 00:00:00'
  ),
  (
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    3000004, true, '0xaaa0000000000000000000000000000000000000000000000000000000000004',
    'fan_four', 'fan_four', '0xAAAA000000000000000000000000000000000004',
    'Fan Four', NULL, NULL,
    false, false, true, false, false,
    '2021-04-01 00:00:00', '2021-04-01 00:00:00'
  ),
  (
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    3000005, true, '0xaaa0000000000000000000000000000000000000000000000000000000000005',
    'fan_five', 'fan_five', '0xAAAA000000000000000000000000000000000005',
    'Fan Five', NULL, NULL,
    false, false, true, false, false,
    '2021-05-01 00:00:00', '2021-05-01 00:00:00'
  );

-- ============================================================
-- TRACKS
-- 2000001: user1 original track
-- 2000002: user1 second track (unlisted)
-- 2000003: user2 track (remix of 2000001)
-- 2000004: user3 track
-- ============================================================

INSERT INTO tracks (
  blockhash, blocknumber, track_id, is_current, is_delete, txhash,
  owner_id, title, duration, genre, mood, tags,
  track_cid, cover_art_sizes,
  track_segments, is_downloadable, is_original_available,
  is_unlisted, is_scheduled_release, is_stream_gated, is_download_gated,
  is_available, is_playlist_upload, is_owned_by_user,
  remix_of, stem_of,
  comments_disabled, no_ai_use,
  created_at, updated_at
) VALUES
  (
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    2000001, true, false,
    '0xbbb0000000000000000000000000000000000000000000000000000000000001',
    3000001, 'Sunrise Drive', 210, 'Electronic', 'Energizing', 'electronic,synth',
    'baeaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa01',
    'baeaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa02',
    '[]', true, false,
    false, false, false, false,
    true, false, true,
    NULL, NULL,
    false, false,
    '2021-06-01 00:00:00', '2021-06-01 00:00:00'
  ),
  (
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    2000002, true, false,
    '0xbbb0000000000000000000000000000000000000000000000000000000000002',
    3000001, 'Hidden Gem (Unlisted)', 180, 'Electronic', 'Melancholic', NULL,
    'baeaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa03',
    NULL,
    '[]', false, false,
    true, false, false, false,
    true, false, true,
    NULL, NULL,
    false, false,
    '2021-06-15 00:00:00', '2021-06-15 00:00:00'
  ),
  (
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    2000003, true, false,
    '0xbbb0000000000000000000000000000000000000000000000000000000000003',
    3000002, 'Sunrise Drive (Remix)', 195, 'Electronic', 'Energizing', 'remix,electronic',
    'baeaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa04',
    'baeaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa05',
    '[]', false, false,
    false, false, false, false,
    true, false, false,
    '{"tracks": [{"parent_track_id": 2000001}]}', NULL,
    false, false,
    '2021-07-01 00:00:00', '2021-07-01 00:00:00'
  ),
  (
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    2000004, true, false,
    '0xbbb0000000000000000000000000000000000000000000000000000000000004',
    3000003, 'Blue Note Sessions', 320, 'Jazz', 'Peaceful', 'jazz,live',
    'baeaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa06',
    'baeaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa07',
    '[]', true, true,
    false, false, false, false,
    true, false, true,
    NULL, NULL,
    false, false,
    '2021-08-01 00:00:00', '2021-08-01 00:00:00'
  );

-- ============================================================
-- PLAYLISTS
-- 400001: user1 album (contains tracks 2000001, 2000002)
-- 400002: user4 playlist (contains tracks 2000001, 2000004)
-- ============================================================

INSERT INTO playlists (
  blockhash, blocknumber, playlist_id, is_current, is_delete, txhash,
  playlist_owner_id, playlist_name, is_album, is_private,
  is_scheduled_release, is_stream_gated, is_image_autogenerated,
  playlist_contents, description,
  created_at, updated_at
) VALUES
  (
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    400001, true, false,
    '0xccc0000000000000000000000000000000000000000000000000000000000001',
    3000001, 'Sunrise EP', true, false,
    false, false, false,
    '{"track_ids": [{"track": 2000001, "timestamp": 1622505600}, {"track": 2000002, "timestamp": 1623715200}]}',
    'My debut EP',
    '2021-09-01 00:00:00', '2021-09-01 00:00:00'
  ),
  (
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    400002, true, false,
    '0xccc0000000000000000000000000000000000000000000000000000000000002',
    3000004, 'My Favorites', false, false,
    false, false, false,
    '{"track_ids": [{"track": 2000001, "timestamp": 1622505600}, {"track": 2000004, "timestamp": 1628812800}]}',
    'Tracks I keep coming back to',
    '2021-10-01 00:00:00', '2021-10-01 00:00:00'
  );

-- ============================================================
-- FOLLOWS
-- user4 follows user1, user2, user3
-- user5 follows user1
-- ============================================================

INSERT INTO follows (
  blockhash, blocknumber, follower_user_id, followee_user_id,
  is_current, is_delete, txhash, created_at
) VALUES
  ('0x0000000000000000000000000000000000000000000000000000000000000001', 1,
   3000004, 3000001, true, false,
   '0xddd0000000000000000000000000000000000000000000000000000000000001',
   '2021-10-02 00:00:00'),
  ('0x0000000000000000000000000000000000000000000000000000000000000001', 1,
   3000004, 3000002, true, false,
   '0xddd0000000000000000000000000000000000000000000000000000000000002',
   '2021-10-02 01:00:00'),
  ('0x0000000000000000000000000000000000000000000000000000000000000001', 1,
   3000004, 3000003, true, false,
   '0xddd0000000000000000000000000000000000000000000000000000000000003',
   '2021-10-02 02:00:00'),
  ('0x0000000000000000000000000000000000000000000000000000000000000001', 1,
   3000005, 3000001, true, false,
   '0xddd0000000000000000000000000000000000000000000000000000000000004',
   '2021-10-03 00:00:00');

-- ============================================================
-- REPOSTS
-- user4 reposts track 2000001
-- user5 reposts playlist 400002
-- ============================================================

INSERT INTO reposts (
  blockhash, blocknumber, user_id, repost_item_id, repost_type,
  is_current, is_delete, is_repost_of_repost, txhash, created_at
) VALUES
  ('0x0000000000000000000000000000000000000000000000000000000000000001', 1,
   3000004, 2000001, 'track',
   true, false, false,
   '0xeee0000000000000000000000000000000000000000000000000000000000001',
   '2021-10-04 00:00:00'),
  ('0x0000000000000000000000000000000000000000000000000000000000000001', 1,
   3000005, 400002, 'playlist',
   true, false, false,
   '0xeee0000000000000000000000000000000000000000000000000000000000002',
   '2021-10-05 00:00:00');

-- ============================================================
-- SAVES
-- user4 saves track 2000004
-- user5 saves album 400001
-- ============================================================

INSERT INTO saves (
  blockhash, blocknumber, user_id, save_item_id, save_type,
  is_current, is_delete, is_save_of_repost, txhash, created_at
) VALUES
  ('0x0000000000000000000000000000000000000000000000000000000000000001', 1,
   3000004, 2000004, 'track',
   true, false, false,
   '0xfff0000000000000000000000000000000000000000000000000000000000001',
   '2021-10-06 00:00:00'),
  ('0x0000000000000000000000000000000000000000000000000000000000000001', 1,
   3000005, 400001, 'album',
   true, false, false,
   '0xfff0000000000000000000000000000000000000000000000000000000000002',
   '2021-10-07 00:00:00');

-- ============================================================
-- PLAYS
-- Mix of identified (user_id set) and anonymous plays
-- ============================================================

INSERT INTO plays (user_id, play_item_id, source, created_at, updated_at, signature) VALUES
  (3000004, 2000001, 'feed',      '2021-10-08 10:00:00', '2021-10-08 10:00:00', 'sig-play-0001'),
  (3000005, 2000001, 'search',    '2021-10-08 11:00:00', '2021-10-08 11:00:00', 'sig-play-0002'),
  (3000004, 2000004, 'profile',   '2021-10-08 12:00:00', '2021-10-08 12:00:00', 'sig-play-0003'),
  (NULL,    2000003, 'embed',     '2021-10-09 09:00:00', '2021-10-09 09:00:00', 'sig-play-0004'),
  (3000005, 2000004, 'trending',  '2021-10-09 14:00:00', '2021-10-09 14:00:00', 'sig-play-0005');

-- ============================================================
-- COMMENTS
-- 4000001: user4 top-level comment on track 2000001
-- 4000002: user5 reply to 4000001 (parent_comment_id is tracked in thread table,
--          but comment row itself just references the same entity)
-- ============================================================

INSERT INTO comments (
  comment_id, text, user_id, entity_id, entity_type,
  track_timestamp_s, is_delete, is_visible, is_edited,
  txhash, blockhash, blocknumber, created_at, updated_at
) VALUES
  (
    4000001, 'This track is incredible, love the drop at 1:30!',
    3000004, 2000001, 'Track',
    NULL, false, true, false,
    '0x1110000000000000000000000000000000000000000000000000000000000001',
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    '2021-10-10 08:00:00', '2021-10-10 08:00:00'
  ),
  (
    4000002, 'Agreed, that synth progression is fire',
    3000005, 2000001, 'Track',
    NULL, false, true, false,
    '0x1110000000000000000000000000000000000000000000000000000000000002',
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    '2021-10-10 09:00:00', '2021-10-10 09:00:00'
  );

-- ============================================================
-- COMMENT REACTIONS
-- user5 reacts to user4's comment
-- ============================================================

INSERT INTO comment_reactions (
  comment_id, user_id, is_delete,
  txhash, blockhash, blocknumber, created_at, updated_at
) VALUES
  (
    4000001, 3000005, false,
    '0x2220000000000000000000000000000000000000000000000000000000000001',
    '0x0000000000000000000000000000000000000000000000000000000000000001', 1,
    '2021-10-10 10:00:00', '2021-10-10 10:00:00'
  );

-- ============================================================
-- TIP REACTIONS (reactions table)
-- user4 reacts (reaction_value=1) to a tip tx signed by user5's wallet
-- reaction_type = 'tip', reacted_to = tip transaction signature
-- ============================================================

INSERT INTO reactions (
  reaction_value, sender_wallet, reaction_type, reacted_to, timestamp, blocknumber
) VALUES
  (
    1,
    '0xAAAA000000000000000000000000000000000004',
    'tip',
    'tip-tx-signature-0000000000000000000000000000000000000000000000001',
    '2021-10-11 12:00:00',
    1
  );
