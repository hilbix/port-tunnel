// JavaScript Circular Object Notation V0.0.0
// vim: ft=javascript
//
// JSON, with a twist for circular objects.
//
// Note that this is a clean room reimplementation what I saw in the output of ChatGPT.
// AI says, it stems somewhat from React, however I doubt this.
//
// By purpose this is free as free beer, free speech and free baby.
// (Use babel to translate to ES6, as it uses a few later JS idioms)
'use strict';

var JSCON =
  { stringify:	function(o) { return JSON.stringify(JSCON.serialize(o))	}
  , parse:	function(s) { return JSCON.deserialize(JSON.parse(s))	}
  , isInteger:	function(i) { return Number.isInteger(i) }
  , isString:	function(s) { return s?.constructor === String }
  , isArray:	function(o) { return Array.isArray(o) }
  , isObject:	function(o) { return typeof o === 'object' && (Object.getPrototypeOf(o || 0) || Object.prototype) === Object.prototype }
  , isJSCON:	function(o) { if (JSCON.isArray(o) && JSCON.isObject(o[0])) for (const _ in o[0]) if (_.startsWith('_')) return '_' }
  , serialize:	function(o)
    {
      throw 'not yet implemented';
    }
  , dump:	function(o)
    {
      const map	= new Map();

      return JSON.stringify(o, function(k,v)
        {
          switch (typeof v)
            {
            case 'symbol':	return v.toString();
            case 'object':	if (map.has(v)) return `${map.get(v)}`; map.set(v,`${map.get(this) ?? '$REF'}.${k}`); break;
            }
          return v;
          return v;
        }, 2);
    }
  , deserialize: function(s) {
      const p	= JSCON.isJSCON(s);	// returns the used prefix, which is '_' in this version
      if (!p) throw 'not JSCON';	// looks like this is not a JSCON object, just return as-is

      const ret	= s.map(_ => JSCON.isArray(_) ? [] : JSCON.isObject(_) ? {} : _);
      const map	= new Map();
      const set	= _ => { const s = Symbol.for(_); map.set(_, s); return s };
      const val	= _ => JSCON.isInteger(_) && _>=0 ? ret[_] : map.has(_) ? map.get(_) : set(_);
      const key	= (_,n) =>
        {
          if (!_.startsWith(p))	throw `${n}: parse error: ${_}`;		// this object apparently is not well formed
          const i = parseInt(_.substr(p.length));
          if (`${p}${i}` !== _)	throw `${n}: index error: ${_}`;
          const k = ret[i];
          if (!JSCON.isString(k))	throw `${n}: ref error ${i}: ${k}`;
          return k;
        };

      s.forEach((_,i) =>
        {
          const o	= ret[i];
          if (JSCON.isArray(_))
            _.forEach((_,j) => o[j] = val(_,i));
          else if (JSCON.isObject(_))
            Object.entries(_).forEach(([k,v]) => o[key(k,i)] = val(v,i));
        });
      return ret[0];
    }
  };

// dummy to stream input lines into something human readable
if (typeof module !== 'undefined' && typeof window === 'undefined')
  {
    const rl	= require('readline');
    const r	= rl.createInterface(
      { input:	process.stdin
      , output:	process.stdout
      , terminal: false
      });

    r.on('line', _ =>
      {
        const j = JSON.parse(_);
        const o = JSCON.deserialize(j);
//        console.debug('input:', _);
        console.log(JSCON.dump(o))
      });
  }

