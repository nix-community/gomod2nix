# Parse go.mod in Nix
# Returns a Nix structure with the contents of the go.mod passed in
# in normalised form.

let
  inherit (builtins) elemAt mapAttrs split foldl' match filter typeOf hasAttr length;

  # Strip lines with comments & other junk
  stripStr = s: elemAt (split "^ *" (elemAt (split " *$" s) 0)) 2;
  stripLines = initialLines: foldl' (acc: f: f acc) initialLines [
    # Strip comments
    (lines: map
      (l: stripStr (elemAt (splitString "//" l) 0))
      lines)

    # Strip leading tabs characters
    (lines: map (l: elemAt (match "(\t)?(.*)" l) 1) lines)

    # Filter empty lines
    (filter (l: l != ""))
  ];

  # Parse lines into a structure
  parseLines = lines: (foldl'
    (acc: l:
      let
        m = match "([^ )]*) *(.*)" l;
        directive = elemAt m 0;
        rest = elemAt m 1;

        # Maintain parser state (inside parens or not)
        inDirective =
          if rest == "(" then directive
          else if rest == ")" then null
          else acc.inDirective
        ;

      in
      {
        data = (acc.data // (
          if directive == "" && rest == ")" then { }
          else if inDirective != null && rest == "(" && ! hasAttr inDirective acc.data then {
            ${inDirective} = { };
          }
          else if rest == "(" || rest == ")" then { }
          else if inDirective != null then {
            ${inDirective} = acc.data.${inDirective} // { ${directive} = rest; };
          } else if directive == "replace" then
            (
              let
                segments = split " => " rest;
                getSegment = elemAt segments;
              in
              assert length segments == 3; {
                replace = acc.data.replace // {
                  ${getSegment 0} = "=> ${getSegment 2}";
                };
              }
            )
          else {
            ${directive} = rest;
          }
        )
        );
        inherit inDirective;
      })
    {
      inDirective = null;
      data = {
        require = { };
        replace = { };
        exclude = { };
      };
    }
    lines
  ).data;

  normaliseDirectives = data: (
    let
      normaliseString = s:
        let
          m = builtins.match "([^ ]+) (.+)" s;
        in
        {
          ${elemAt m 0} = elemAt m 1;
        };
      require = data.require or { };
      replace = data.replace or { };
      exclude = data.exclude or { };
    in
    data // {
      require =
        if typeOf require == "string" then normaliseString require
        else require;
      replace =
        if typeOf replace == "string" then normaliseString replace
        else replace;
    }
  );

  parseReplace = data: (
    data // {
      replace =
        mapAttrs
          (_: v:
            let
              m = match "=> ([^ ]+) (.+)" v;
              m2 = match "=> (.*+)" v;
            in
            if m != null then {
              goPackagePath = elemAt m 0;
              version = elemAt m 1;
            } else {
              path = elemAt m2 0;
            })
          data.replace;
    }
  );

  splitString = sep: s: filter (t: t != [ ]) (split sep s);

in
contents:
foldl' (acc: f: f acc) (splitString "\n" contents) [
  stripLines
  parseLines
  normaliseDirectives
  parseReplace
]
