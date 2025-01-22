import React from 'react';

const TitleBar: React.FC = () => {
  return (
    <div 
          className="app-title-bar text-center"
    >
      <img 
        src='/logo.png'
              className='w-10 h-10 bg-auto'
      />
      bili-FM
    </div>
  );
};

export default TitleBar; 