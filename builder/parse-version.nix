let
  inherit (builtins) elemAt match hasAttr removeAttrs;
in
ver:
removeAttrs
  (builtins.foldl' (acc: f: if hasAttr "rest" acc then f acc else acc)
    {
      version = "";
      rev = "";
      versionSuffix = "";
      date = "";
      rest = ver;
    } [
    (acc:
      let
        m = match "([^-]+)-(.*)" acc.rest;
        e = elemAt m;
      in
      if m != null then {
        version = e 0;
        rest = e 1;
      } else removeAttrs acc [ "rest" ] // {
        version = ver;
      })
    (acc:
      let
        m = elemAt (match "(.*)-(.*)" acc.rest);
      in
      acc // {
        rev = m 1;
        rest = m 0;
      })
    (acc:
      let
        m = elemAt (match "(.*)\\.(.*)" acc.rest);
      in
      acc // {
        versionSuffix = m 0;
        date = m 1;
      })
  ]) [ "rest" ]
